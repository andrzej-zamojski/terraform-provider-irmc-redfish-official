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

package models

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// OnlineUpdateResourceModel describes the resource data model.
type OnlineUpdateResourceModel struct {
	Id            types.String    `tfsdk:"id"`
	RedfishServer []RedfishServer `tfsdk:"server"`

	// Prepare Online Update
	UpdateList types.List `tfsdk:"update_list"`

	//Execute Online Update
	ExecuteOnlineUpdOperationTime types.String `tfsdk:"execute_online_upd_operation_time"`
	ExecuteOnlineUpdScheduleTime  types.String `tfsdk:"execute_online_upd_schedule_time"`
}

type OnlineUpdateCheckItem struct {
	Designation     types.String `tfsdk:"designation"`
	Component       types.String `tfsdk:"component"`
	SubComponent    types.String `tfsdk:"sub_component"`
	CurrentVersion  types.String `tfsdk:"current_version"`
	NewVersion      types.String `tfsdk:"new_version"`
	Severity        types.String `tfsdk:"severity"`
	Status          types.String `tfsdk:"status"`
	RebootRequired  types.String `tfsdk:"reboot_required"`
	Downloaded      types.Bool   `tfsdk:"downloaded"`
	ExecutionStatus types.String `tfsdk:"execution_status"`
	RelNotePath     types.String `tfsdk:"release_note_path"`
}

type OnlineUpdateCheck struct {
	LastStatusChangeDate types.String            `tfsdk:"last_status_change_date"`
	UpdateCollection     []OnlineUpdateCheckItem `tfsdk:"update_collection"`
}

type OnlineUpdateDataSourceModel struct {
	Id            types.String    `tfsdk:"id"`
	RedfishServer []RedfishServer `tfsdk:"server"`
	OnlineUpdateCheck
}
