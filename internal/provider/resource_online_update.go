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

package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"terraform-provider-irmc-redfish/internal/models"
	"terraform-provider-irmc-redfish/internal/validators"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/stmcginnis/gofish"
)

var allowedUpdateComponents = map[string]struct{}{
	"Agent-Lx":               {},
	"Agent-Win":              {},
	"FibreChannelController": {},
	"LanController":          {},
	"ManagementController":   {},
	"PrimSupportPack-Win":    {},
	"ScsiController":         {},
	"Storage":                {},
	"SystemBoard":            {},
	"Others":                 {},
}

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &OnlineUpdateResource{}

func NewOnlineUpdateResource() resource.Resource {
	return &OnlineUpdateResource{}
}

// OnlineUpdateResource defines the resource implementation.
type OnlineUpdateResource struct {
	p *IrmcProvider
}

func (r *OnlineUpdateResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + onlineUpdate
}

func (r *OnlineUpdateResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the Online Update execution process on an IRMC server.",
		Description:         "Manages the Online Update execution process on an IRMC server.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "ID of the online update execution endpoint.",
				Description:         "ID of the online update execution endpoint.",
				Computed:            true,
			},
			"update_list": schema.ListAttribute{
				ElementType:         types.StringType,
				MarkdownDescription: "List of updates to apply. Can be component types (e.g., `SystemBoard`), specific designations (e.g., `SystemBoard/D3988-A1`), or the keyword `Others`. If empty or null, applies all available updates found and marked for execution.",
				Description:         "List of updates to apply. Can be component types (e.g., `SystemBoard`), specific designations (e.g., `SystemBoard/D3988-A1`), or the keyword `Others`. If empty or null, applies all available updates found and marked for execution.",
				Optional:            true,
				Computed:            true,
			},
			"execute_online_upd_operation_time": schema.StringAttribute{
				MarkdownDescription: "When the update execution should occur. Allowed values: `Immediately`, `Once`. Defaults to `Immediately`.",
				Description:         "When the update execution should occur. Allowed values: `Immediately`, `Once`. Defaults to `Immediately`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("Immediately"),
				Validators: []validator.String{
					stringvalidator.OneOf("Immediately", "Once"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"execute_online_upd_schedule_time": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Required if `execute_online_upd_operation_time` is `Once`. Specifies the date and time for the scheduled execution (check API docs for exact format).",
				Description:         "Required date/time for `Once` execution.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					validators.ChangeToRequired("execute_online_upd_operation_time", "Once"),
				},
			},
		},
		Blocks: RedfishServerResourceBlockMap(),
	}
}

func (r *OnlineUpdateResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	p, ok := req.ProviderData.(*IrmcProvider)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *IrmcProvider, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.p = p
}

func (r *OnlineUpdateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "resource-online-update: create starts")
	var plan models.OnlineUpdateResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := plan.RedfishServer[0].Endpoint.ValueString()
	resourceName := "resource-online-update"
	mutexPool.Lock(ctx, endpoint, resourceName)
	defer mutexPool.Unlock(ctx, endpoint, resourceName)

	api, err := ConnectTargetSystem(r.p, &plan.RedfishServer)
	if err != nil {
		resp.Diagnostics.AddError("Service Connection Error", err.Error())
		return
	}
	defer api.Logout()

	isFsas, err := IsFsasCheck(ctx, api)
	if err != nil {
		resp.Diagnostics.AddError("Vendor Detection Failed", err.Error())
		return
	}

	if err := CheckELCMLicense(api, GetLicenseEndpoint(isFsas)); err != nil {
		resp.Diagnostics.AddError("eLCM License Check Failed", err.Error())
		return
	}

	endpoints := GetOnlineUpdateEndpoints(isFsas)
	var collection *models.OnlineUpdateCheck

	if IsCollectionCacheValid(ctx, api, endpoints.collectionEndpoint) {
		collection, err = GetOnlineUpdateCollectionWithRetry(ctx, api, endpoints.collectionEndpoint, 3, 1*time.Second)
		if err != nil {
			resp.Diagnostics.AddError("Failed to retrieve cached collection", err.Error())
			return
		}
	} else {

		taskLocation, err := TriggerOnlineUpdateCheck(ctx, api, endpoints.checkEndpoint)
		if err != nil {
			resp.Diagnostics.AddError("Trigger Online Update Check Failed", err.Error())
			return
		}

		if taskLocation != "" {
			if err := CheckOnlineUpdateStatus(ctx, api.Service, taskLocation, DEFAULT_ONLINEUPDATE_TIMEOUT, isFsas); err != nil {
				resp.Diagnostics.AddError("Preaper Online Update Check Task Failed", err.Error())
				return
			}
		} else {
			time.Sleep(5 * time.Second)
		}

		collection, err = GetOnlineUpdateCollectionWithRetry(ctx, api, endpoints.collectionEndpoint, 12, 5*time.Second)
		if err != nil {
			resp.Diagnostics.AddError("Collection Retrieval Error after new check", err.Error())
			return
		}
	}

	if len(collection.UpdateCollection) == 0 {
		tflog.Info(ctx, "Online update check completed successfully, but no updates are currently available for this system.")
		plan.Id = types.StringValue(endpoints.checkEndpoint)
		diags = resp.State.Set(ctx, &plan)
		resp.Diagnostics.Append(diags...)
		tflog.Info(ctx, "resource-online-update: create ends (no updates available)")
		return
	}

	selected, deselected, err := PrepareUpdateLists(plan, collection)
	if err != nil {
		resp.Diagnostics.AddError("Failed to prepare update lists", err.Error())
		return
	}

	if err := DeselectUpdates(ctx, api, endpoints.modifyCollectionEndpoint, deselected); err != nil {
		resp.Diagnostics.AddError("Failed to deselect updates via ModifyCollection", err.Error())
		return
	}

	executePayload, err := BuildExecutePayload(plan, isFsas)
	if err != nil {
		resp.Diagnostics.AddError("Failed to build execute payload", err.Error())
		return
	}

	shouldExecute := len(selected) > 0 || plan.UpdateList.IsNull()
	var executeTaskLocation string

	if shouldExecute {
		executeTaskLocation, err = TriggerOnlineUpdateExecute(ctx, api, endpoints.checkEndpoint, executePayload)
		if err != nil {
			resp.Diagnostics.AddError("Trigger Online Update Execute Failed", err.Error())
			return
		}

		operationTimeType := plan.ExecuteOnlineUpdOperationTime.ValueString()

		if operationTimeType == "Immediately" {
			if executeTaskLocation != "" {
				if err := CheckOnlineUpdateStatus(ctx, api.Service, executeTaskLocation, DEFAULT_ONLINEUPDATE_TIMEOUT, isFsas); err != nil {
					resp.Diagnostics.AddError("Online Update Execute Task Failed", err.Error())
					return
				}
			} else {
				time.Sleep(10 * time.Second)
			}
		}
	} else {
		if !plan.UpdateList.IsNull() && len(collection.UpdateCollection) > 0 && len(selected) == 0 && len(deselected) > 0 {
			resp.Diagnostics.AddWarning("No matching updates found", "The specified 'update_list' did not match any available updates in the collection. No updates were executed.")
		}
	}

	plan.Id = types.StringValue(endpoints.checkEndpoint)
	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	tflog.Info(ctx, "resource-online-update: create ends")
}

func (r *OnlineUpdateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Info(ctx, "resource-online-update: read starts")
	var state models.OnlineUpdateResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	tflog.Info(ctx, "resource-online-update: read ends")
}

func (r *OnlineUpdateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "resource-simple-update: update starts")

	// All attributes require the resource to be replaced, the Update operation is not needed.

	tflog.Info(ctx, "resource-simple-update: update ends")
}

func (r *OnlineUpdateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Info(ctx, "resource-online-update: delete starts")
	resp.State.RemoveResource(ctx)
	tflog.Info(ctx, "resource-online-update: delete ends")
}

func DeselectUpdates(ctx context.Context, api *gofish.APIClient, modifyEndpoint string, designationsToDeselect []string) error {
	if len(designationsToDeselect) == 0 {
		return nil
	}

	client := api.Service.GetClient()

	for _, designation := range designationsToDeselect {

		modification := []map[string]string{
			{
				"Designation": designation,
				"Execution":   "deselected",
			},
		}
		payload := map[string]interface{}{
			"UpdateCollectionModifications": modification,
		}

		res, err := client.Post(modifyEndpoint, payload)
		if err != nil {
			return fmt.Errorf("POST request to deselect '%s' failed: %w", designation, err)
		}

		bodyCloseErr := res.Body.Close()
		if bodyCloseErr != nil {
			tflog.Warn(ctx, "Error closing response body during deselect loop", map[string]interface{}{"error": bodyCloseErr.Error()})
		}

		if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
			body, _ := io.ReadAll(res.Body)
			return fmt.Errorf("deselect POST request for '%s' returned status code %d: %s", designation, res.StatusCode, string(body))
		}
	}

	return nil
}

func PrepareUpdateLists(plan models.OnlineUpdateResourceModel, collection *models.OnlineUpdateCheck) ([]string, []string, error) {

	var requestedUpdates []string
	userProvidedList := !plan.UpdateList.IsNull() && !plan.UpdateList.IsUnknown()

	if userProvidedList {
		diags := plan.UpdateList.ElementsAs(context.Background(), &requestedUpdates, false)
		if diags.HasError() {
			return nil, nil, fmt.Errorf("invalid update_list format")
		}
	}
	ctx := context.Background()
	requestedDesignations := make(map[string]struct{})
	requestedComponents := make(map[string]struct{})
	applyOthers := false
	applyAll := true
	noneEmptyRequest := false

	for _, req := range requestedUpdates {
		trimmedReq := strings.TrimSpace(req)
		if trimmedReq == "" {
			continue
		}

		noneEmptyRequest = true

		if trimmedReq == "Others" {
			applyOthers = true
		} else if strings.Contains(trimmedReq, "/") {
			requestedDesignations[trimmedReq] = struct{}{}
		} else if _, ok := allowedUpdateComponents[trimmedReq]; ok {
			requestedComponents[trimmedReq] = struct{}{}
		} else {
			tflog.Warn(ctx, fmt.Sprintf("Ignoring unrecognized item in update_list: '%s'. It is not a known component type, specific designation (like 'Component/Name'), or the keyword 'Others'.", trimmedReq))
		}
	}

	if noneEmptyRequest {
		applyAll = false
	}

	selectedDesignations := []string{}
	deselectedDesignations := []string{}

	for _, item := range collection.UpdateCollection {
		designation := item.Designation.ValueString()
		component := item.Component.ValueString()

		isSelected := false
		if applyAll {
			isSelected = true
		} else {
			if _, ok := requestedDesignations[designation]; ok {
				isSelected = true
			}
			if !isSelected {
				if _, ok := requestedComponents[component]; ok {
					isSelected = true
				}
			}
			if !isSelected && applyOthers {
				if _, ok := allowedUpdateComponents[component]; !ok {
					isSelected = true
				}
			}
		}

		if isSelected {
			selectedDesignations = append(selectedDesignations, designation)
		} else if !applyAll {
			deselectedDesignations = append(deselectedDesignations, designation)
		}
	}

	return selectedDesignations, deselectedDesignations, nil
}

func BuildExecutePayload(plan models.OnlineUpdateResourceModel, isFsas bool) (map[string]interface{}, error) {

	payload := map[string]interface{}{
		"ExecutionMode": "ExecuteUpdate",
	}

	operationTimeType := "Immediately"
	if !plan.ExecuteOnlineUpdOperationTime.IsNull() && !plan.ExecuteOnlineUpdOperationTime.IsUnknown() {
		operationTimeType = plan.ExecuteOnlineUpdOperationTime.ValueString()
	}
	payload["SchedulingType"] = operationTimeType

	if operationTimeType == "Once" {
		if !plan.ExecuteOnlineUpdScheduleTime.IsNull() && !plan.ExecuteOnlineUpdScheduleTime.IsUnknown() {
			payload["StartDate"] = plan.ExecuteOnlineUpdScheduleTime.ValueString()
		} else {
			return nil, fmt.Errorf("attribute 'execute_online_upd_schedule_time' is required when 'execute_online_upd_operation_time' is 'Once'")
		}
	} else if !plan.ExecuteOnlineUpdScheduleTime.IsNull() && !plan.ExecuteOnlineUpdScheduleTime.IsUnknown() {
		tflog.Warn(context.Background(), "'execute_online_upd_schedule_time' is provided but 'execute_online_upd_operation_time' is 'Immediately'. 'execute_online_upd_schedule_time' will be ignored by the API.")
	}

	return payload, nil
}

func TriggerOnlineUpdateExecute(ctx context.Context, api *gofish.APIClient, endpoint string, payload map[string]interface{}) (string, error) {
	client := api.Service.GetClient()

	respPost, err := client.Post(endpoint, payload)
	if err != nil {
		return "", fmt.Errorf("POST request failed: %w", err)
	}
	defer respPost.Body.Close()

	if respPost.StatusCode != http.StatusAccepted && respPost.StatusCode != http.StatusOK && respPost.StatusCode != http.StatusNoContent && respPost.StatusCode != http.StatusCreated {
		bodyBytes, readErr := io.ReadAll(respPost.Body)
		if readErr != nil {
			return "", fmt.Errorf("POST request returned status code %d", respPost.StatusCode)
		}
		return "", fmt.Errorf("POST request returned status code %d: %s", respPost.StatusCode, string(bodyBytes))
	}

	location := respPost.Header.Get(HTTP_HEADER_LOCATION)
	if location == "" {
		return "", fmt.Errorf("Task Location header not found in response")
	}
	return location, nil
}
