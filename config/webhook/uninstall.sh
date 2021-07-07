#!/usr/bin/env bash

# Copyright 2021 The Pixiu Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euo pipefail

# Delete the TLS secret for the generated keys.
kubectl -n pixiu-system delete secret webhook-server-tls

kubectl delete -f pixiu-webhook-server.yaml

echo "The webhook server has been deleted!"
