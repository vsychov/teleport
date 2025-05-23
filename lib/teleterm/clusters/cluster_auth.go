/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package clusters

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sort"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/utils/keys"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	"github.com/gravitational/teleport/lib/client"
	dbprofile "github.com/gravitational/teleport/lib/client/db"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
)

// SyncAuthPreference fetches Teleport auth preferences and stores it in the cluster profile
func (c *Cluster) SyncAuthPreference(ctx context.Context) (*webclient.WebConfigAuthSettings, *webclient.PingResponse, error) {
	pingResponse, err := c.clusterClient.Ping(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	pingResponseJSON, err := json.Marshal(pingResponse)
	if err != nil {
		c.Logger.DebugContext(ctx, "Could not marshal ping response to JSON", "error", err)
	} else {
		c.Logger.DebugContext(ctx, "Got ping response", "response", string(pingResponseJSON))
	}

	if err := c.clusterClient.SaveProfile(false); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	cfg, err := c.clusterClient.GetWebConfig(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return &cfg.Auth, pingResponse, nil
}

// Logout deletes all cluster certificates
func (c *Cluster) Logout(ctx context.Context) error {
	// Delete db certs
	for _, db := range c.status.Databases {
		err := dbprofile.Delete(c.clusterClient, db)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// Remove cluster entries from kubeconfig
	if err := kubeconfig.RemoveByServerAddr("", c.clusterClient.KubeClusterAddr()); err != nil {
		return trace.Wrap(err)
	}

	// Remove keys for this user from disk and running agent.
	if err := c.clusterClient.Logout(); err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	return nil
}

// LocalLogin processes local logins for this cluster
func (c *Cluster) LocalLogin(ctx context.Context, user, password, otpToken string) error {
	c.clusterClient.AuthConnector = constants.LocalConnector

	if err := c.login(ctx, c.localMFALogin(user, password)); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// SSOLogin logs in a user to the Teleport cluster using supported SSO provider
func (c *Cluster) SSOLogin(ctx context.Context, providerType, providerName string) error {
	// Get the ping response for the given auth connector.
	c.clusterClient.AuthConnector = providerName

	if _, err := c.updateClientFromPingResponse(ctx); err != nil {
		return trace.Wrap(err)
	}

	if err := c.login(ctx, c.ssoLogin(providerType, providerName)); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// PasswordlessLogin processes passwordless logins for this cluster.
func (c *Cluster) PasswordlessLogin(ctx context.Context, stream api.TerminalService_LoginPasswordlessServer) error {
	// Get the ping response for the given auth connector.
	c.clusterClient.AuthConnector = constants.PasswordlessConnector

	if _, err := c.updateClientFromPingResponse(ctx); err != nil {
		return trace.Wrap(err)
	}

	if err := c.login(ctx, c.passwordlessLogin(stream)); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (c *Cluster) updateClientFromPingResponse(ctx context.Context) (*webclient.PingResponse, error) {
	pingResp, err := c.clusterClient.Ping(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.clusterClient.KeyTTL = cmp.Or(c.clusterClient.KeyTTL, pingResp.Auth.DefaultSessionTTL.Duration(), defaults.CertDuration)

	return pingResp, nil
}

type SSHLoginFunc func(context.Context, *keys.PrivateKey) (*authclient.SSHLoginResponse, error)

func (c *Cluster) login(ctx context.Context, sshLoginFunc client.SSHLoginFunc) error {
	// TODO(alex-kovoy): SiteName needs to be reset if trying to login to a cluster with
	// existing profile for the first time (investigate why)
	c.clusterClient.SiteName = ""

	key, err := c.clusterClient.SSHLogin(ctx, sshLoginFunc)
	if err != nil {
		return trace.Wrap(err)
	}

	// Update username before updating the profile
	c.clusterClient.LocalAgent().UpdateUsername(key.Username)
	c.clusterClient.Username = key.Username

	proxyClient, rootAuthClient, err := c.clusterClient.ConnectToRootCluster(ctx, key)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		rootAuthClient.Close()
		proxyClient.Close()
	}()

	// Attempt device login. This activates a fresh key if successful.
	if err := c.clusterClient.AttemptDeviceLogin(ctx, key, rootAuthClient); err != nil {
		return trace.Wrap(err)
	}

	if err := c.clusterClient.SaveProfile(true); err != nil {
		return trace.Wrap(err)
	}

	status, err := c.clusterClient.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}

	c.status = *status

	return nil
}

func (c *Cluster) localMFALogin(user, password string) client.SSHLoginFunc {
	return func(ctx context.Context, keyRing *client.KeyRing) (*authclient.SSHLoginResponse, error) {
		sshLogin, err := c.clusterClient.NewSSHLogin(keyRing)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		response, err := client.SSHAgentMFALogin(ctx, client.SSHLoginMFA{
			SSHLogin:             sshLogin,
			User:                 user,
			Password:             password,
			MFAPromptConstructor: c.clusterClient.NewMFAPrompt,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return response, nil
	}
}

func (c *Cluster) ssoLogin(providerType, providerName string) client.SSHLoginFunc {
	return c.clusterClient.SSOLoginFn(providerName, providerName, providerType)
}

func (c *Cluster) passwordlessLogin(stream api.TerminalService_LoginPasswordlessServer) client.SSHLoginFunc {
	return func(ctx context.Context, keyRing *client.KeyRing) (*authclient.SSHLoginResponse, error) {
		sshLogin, err := c.clusterClient.NewSSHLogin(keyRing)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		response, err := client.SSHAgentPasswordlessLogin(ctx, client.SSHLoginPasswordless{
			SSHLogin:                sshLogin,
			AuthenticatorAttachment: c.clusterClient.AuthenticatorAttachment,
			CustomPrompt:            newPwdlessLoginPrompt(ctx, c.Logger, stream),
			WebauthnLogin:           c.clusterClient.WebauthnLogin,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return response, nil
	}
}

// pwdlessLoginPrompt is a implementation for wancli.LoginPrompt for teleterm passwordless logins.
type pwdlessLoginPrompt struct {
	log    *slog.Logger
	Stream api.TerminalService_LoginPasswordlessServer
}

func newPwdlessLoginPrompt(ctx context.Context, log *slog.Logger, stream api.TerminalService_LoginPasswordlessServer) *pwdlessLoginPrompt {
	return &pwdlessLoginPrompt{
		log:    log,
		Stream: stream,
	}
}

// PromptPIN prompts the user for a PIN.
func (p *pwdlessLoginPrompt) PromptPIN() (string, error) {
	if err := p.Stream.Send(&api.LoginPasswordlessResponse{
		Prompt: api.PasswordlessPrompt_PASSWORDLESS_PROMPT_PIN,
	}); err != nil {
		return "", trace.Wrap(err)
	}

	req, err := p.Stream.Recv()
	if err != nil {
		return "", trace.Wrap(err)
	}

	pinRes := req.GetPin()
	if pinRes == nil || pinRes.GetPin() == "" {
		return "", trace.BadParameter("pin is required")
	}

	return pinRes.GetPin(), nil
}

// PromptTouch prompts the user for a security key touch.
func (p *pwdlessLoginPrompt) PromptTouch() (wancli.TouchAcknowledger, error) {
	return p.ackTouch, trace.Wrap(p.Stream.Send(&api.LoginPasswordlessResponse{Prompt: api.PasswordlessPrompt_PASSWORDLESS_PROMPT_TAP}))
}

func (p *pwdlessLoginPrompt) ackTouch() error {
	// TODO(nklaassen): Send touch acknowledgement if worth the effort, this is
	// not strictly necessary but a nice-to-have acknowledgement to the user
	// that we successfully detected their tap.
	// The current gRPC message type switch in teleterm client code will reject
	// any new message types, making this difficult to add without breaking
	// older clients.
	p.log.DebugContext(context.Background(), "Detected security key tap")
	return nil
}

// PromptCredential prompts the user to select a login name in the list of logins.
func (p *pwdlessLoginPrompt) PromptCredential(deviceCreds []*wancli.CredentialInfo) (*wancli.CredentialInfo, error) {
	// Shouldn't happen, but let's check just in case.
	if len(deviceCreds) == 0 {
		return nil, errors.New("attempted to prompt credential with empty credentials")
	}

	// Sorts in place.
	sort.Slice(deviceCreds, func(i, j int) bool {
		c1 := deviceCreds[i]
		c2 := deviceCreds[j]
		return c1.User.Name < c2.User.Name
	})

	// Convert to grpc message.
	creds := make([]*api.CredentialInfo, len(deviceCreds))
	for i, cred := range deviceCreds {
		creds[i] = &api.CredentialInfo{
			Username: cred.User.Name,
		}
	}

	if err := p.Stream.Send(&api.LoginPasswordlessResponse{
		Prompt:      api.PasswordlessPrompt_PASSWORDLESS_PROMPT_CREDENTIAL,
		Credentials: creds,
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	req, err := p.Stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	credRes := req.GetCredential()
	if credRes == nil {
		return nil, trace.BadParameter("login name must be selected")
	}

	// Test for out of range index values.
	selectedIndex := credRes.GetIndex()
	if selectedIndex < 0 || selectedIndex > int64(len(creds))-1 {
		return nil, trace.BadParameter("invalid login name")
	}

	return deviceCreds[selectedIndex], nil
}
