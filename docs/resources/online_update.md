<!--
Copyright (c) 2025 Fsas Technologies Inc., or its subsidiaries. All Rights Reserved.

Licensed under the Mozilla Public License Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://mozilla.org/MPL/2.0/

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
-->

# irmc-redfish_online_update (Resource)

Manages the Online Update execution process on an IRMC server.


## Schema

### Optional

- `execute_online_upd_operation_time` (String) When the update execution should occur. Allowed values: `Immediately`, `Once`. Defaults to `Immediately`.
- `execute_online_upd_schedule_time` (String) Required if `execute_online_upd_operation_time` is `Once`. Specifies the date and time for the scheduled execution (check API docs for exact format).
- `server` (Block List) List of server BMCs and their respective user credentials (see [below for nested schema](#nestedblock--server))
- `update_list` (List of String) List of updates to apply. Can be component types (e.g., `SystemBoard`), specific designations (e.g., `SystemBoard/D3988-A1`), or the keyword `Others`. If empty or null, applies all available updates found and marked for execution.

### Read-Only

- `id` (String) ID of the online update execution endpoint.

<a id="nestedblock--server"></a>
### Nested Schema for `server`

Required:

- `endpoint` (String) Server BMC IP address or hostname

Optional:

- `password` (String, Sensitive) User password for login
- `ssl_insecure` (Boolean) This field indicates whether the SSL/TLS certificate must be verified or not
- `username` (String) User name for login
