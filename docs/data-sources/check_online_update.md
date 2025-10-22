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

# irmc-redfish_check_online_update (Data Source)

Check online update check data source


## Schema

### Optional

- `server` (Block List) List of server BMCs and their respective user credentials (see [below for nested schema](#nestedblock--server))

### Read-Only

- `id` (String) ID of the online update check endpoint.
- `last_status_change_date` (String) Last time the online update check status changed.
- `update_collection` (Attributes List) (see [below for nested schema](#nestedatt--update_collection))

<a id="nestedblock--server"></a>
### Nested Schema for `server`

Required:

- `endpoint` (String) Server BMC IP address or hostname

Optional:

- `password` (String, Sensitive) User password for login
- `ssl_insecure` (Boolean) This field indicates whether the SSL/TLS certificate must be verified or not
- `username` (String) User name for login


<a id="nestedatt--update_collection"></a>
### Nested Schema for `update_collection`

Read-Only:

- `component` (String)
- `current_version` (String)
- `designation` (String)
- `downloaded` (Boolean)
- `execution_status` (String)
- `new_version` (String)
- `reboot_required` (String)
- `release_note_path` (String)
- `severity` (String)
- `status` (String)
- `sub_component` (String)
