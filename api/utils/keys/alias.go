/*
Copyright 2025 Gravitational, Inc.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package keys

import "github.com/gravitational/teleport/api/utils/keys/hardwarekey"

// Temporary aliases for types moved to the hardwarekey or piv packages
// TODO(Joerger): Remove once /e no longer relies on them.

// AttestationStatement is an attestation statement for a hardware private key
// that supports json marshaling through the standard json/encoding package.
type AttestationStatement = hardwarekey.AttestationStatement

// AttestationStatementFromProto converts an AttestationStatement from its protobuf form.
var AttestationStatementFromProto = hardwarekey.AttestationStatementFromProto
