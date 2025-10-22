/*
Copyright (c) 2024 Fsas Technologies Inc., or its subsidiaries. All Rights Reserved.

Licensed under the Mozilla Public License Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://mozilla.org/MPL/2.0/


Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

resource "irmc-redfish_online_update" "o_update" {
  for_each = var.rack1
  server {
    username     = each.value.username
    password     = each.value.password
    endpoint     = each.value.endpoint
    ssl_insecure = each.value.ssl_insecure
  }

  // User can use whole designation string or use one of below components update types:
  // "Agent-Lx", "Agent-Win", "FibreChannelController", "LanController", 
  // "ManagementController", "PrimSupportPack-Win", "ScsiController", "Storage", "SystemBoard", "Others"
  
  update_list = ["ManagementController/iRMC%20S6-RX2540M7-0666"]
  
  
  //execute_online_upd_operation_time = "Once"
  // execute_online_upd_schedule_time = "23:00 10.04.2026"  // Required if execute_online_upd_operation_time is "Once"
}
