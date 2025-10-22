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

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure IrmcProvider satisfies various provider interfaces.
var _ provider.Provider = &IrmcProvider{}

var mutexPool = InitSyncPoolInstance()

// IrmcProvider defines the provider implementation.
type IrmcProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string

	Username string
	Password string
}

// IrmcProviderModel describes the provider data model.
type IrmcProviderModel struct {
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
}

func (p *IrmcProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	// Here is provider name -------------------
	resp.TypeName = "irmc-redfish_"
	// Above is provider name ------------------

	resp.Version = p.version
}

func (p *IrmcProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"username": schema.StringAttribute{
				MarkdownDescription: "Username accessing Redfish API",
				Description:         "Username accessing Redfish API",
				Optional:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Password related to given user name accessing Redfish API",
				Description:         "Password related to given user name accessing Redfish API",
				Optional:            true,
			},
		},
	}
}

func (p *IrmcProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data IrmcProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Configuration values are now available.
	if data.Username.IsUnknown() {
		resp.Diagnostics.AddWarning(
			"Unable to create client as username is missing",
			"Cannot use unknown value",
		)
	}

	if data.Password.IsUnknown() {
		resp.Diagnostics.AddWarning(
			"Unable to create client as password is missing",
			"Cannot use unknown value",
		)
	}

	p.Username = data.Username.ValueString()
	p.Password = data.Password.ValueString()

	resp.ResourceData = p
	resp.DataSourceData = p

	tflog.Trace(ctx, "Finished configuring the provider")

	// Example client configuration for data sources and resources
	/*
		client := http.DefaultClient
		resp.DataSourceData = client
		resp.ResourceData = client
	*/
}

func (p *IrmcProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewVirtualMediaResource,
		NewPowerResource,
		NewIrmcRestartResource,
		NewBootSourceOverrideResource,
		NewBootOrderResource,
		NewBiosResource,
		NewUserAccountResource,
		NewSimpleUpdateResource,
		NewStorageResource,
		NewStorageVolumeResource,
		NewIrmcFirmwareUpdateResource,
		NewIrmcAttributesResource,
		NewIrmcCertificateCaUpdDeployResource,
		NewIrmcCertificateWebServerResource,
		NewIrmcCertificateCaCasSmtpResource,
		NewOnlineUpdateResource,
	}
}

func (p *IrmcProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewVirtualMediaDataSource,
		NewBiosDataSource,
		NewFirmwareInventoryDataSource,
		NewStorageDataSource,
		NewSystemBootDataSource,
		NewIrmcAttributesDataSource,
		NewOnlineUpdateDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &IrmcProvider{
			version: version,
		}
	}
}
