/*
Copyright (c) 2025 Fsas Technologies Inc.,
or its subsidiaries. All Rights Reserved.

Licensed under the Mozilla Public License Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://mozilla.org/MPL/2.0/

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
either express or implied.
*/

package provider

import (
	"context"
	"fmt"
	"terraform-provider-irmc-redfish/internal/models"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const DEFAULT_ONLINEUPDATE_TIMEOUT = int64(6000)

var _ datasource.DataSource = &OnlineUpdateDataSource{}

func NewOnlineUpdateDataSource() datasource.DataSource {
	return &OnlineUpdateDataSource{}
}

type OnlineUpdateDataSource struct {
	p *IrmcProvider
}

func (d *OnlineUpdateDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + checkonlineUpdate
}

func OnlineUpdateDataSourceSchema() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "ID of the online update check endpoint.",
		},
		"last_status_change_date": schema.StringAttribute{
			Computed:    true,
			Description: "Last time the online update check status changed.",
		},
		"update_collection": schema.ListNestedAttribute{
			Computed: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"designation":       schema.StringAttribute{Computed: true},
					"component":         schema.StringAttribute{Computed: true},
					"sub_component":     schema.StringAttribute{Computed: true},
					"current_version":   schema.StringAttribute{Computed: true},
					"new_version":       schema.StringAttribute{Computed: true},
					"severity":          schema.StringAttribute{Computed: true},
					"status":            schema.StringAttribute{Computed: true},
					"reboot_required":   schema.StringAttribute{Computed: true},
					"downloaded":        schema.BoolAttribute{Computed: true},
					"execution_status":  schema.StringAttribute{Computed: true},
					"release_note_path": schema.StringAttribute{Computed: true},
				},
			},
		},
	}
}

func (d *OnlineUpdateDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Check online update check data source",
		Attributes:          OnlineUpdateDataSourceSchema(),
		Blocks:              RedfishServerDatasourceBlockMap(),
	}
}

func (d *OnlineUpdateDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	p, ok := req.ProviderData.(*IrmcProvider)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *http.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.p = p
}

func (d *OnlineUpdateDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	tflog.Info(ctx, "data-online-update: read starts")

	var data models.OnlineUpdateDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	api, err := ConnectTargetSystem(d.p, &data.RedfishServer)
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
		tflog.Info(ctx, "Using cached online update collection.")
		collection, err = GetOnlineUpdateCollectionWithRetry(ctx, api, endpoints.collectionEndpoint, 5, 5*time.Second)
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
				resp.Diagnostics.AddError("Online Update Task Failed", err.Error())
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

	data.Id = types.StringValue(endpoints.checkEndpoint)
	data.LastStatusChangeDate = collection.LastStatusChangeDate
	data.UpdateCollection = collection.UpdateCollection

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	tflog.Info(ctx, "data-online-update: read ends")
}
