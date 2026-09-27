package provider

import (
	"context"
	"fmt"

	libvirtclient "github.com/dmacvicar/terraform-provider-libvirt/v2/internal/libvirt"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"libvirt.org/go/libvirtxml"
)

var _ resource.Resource = &NWFilterResource{}
var _ resource.ResourceWithConfigure = &NWFilterResource{}

type NWFilterResource struct {
	client *libvirtclient.Client
}

type NWFilterResourceModel struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Chain    types.String `tfsdk:"chain"`
	Priority types.Int64  `tfsdk:"priority"`
}

func NewNWFilterResource() resource.Resource {
	return &NWFilterResource{}
}

func (r *NWFilterResource) Metadata(
	ctx context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_nwfilter"
}

func (r *NWFilterResource) Schema(
	ctx context.Context,
	req resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		Description: "Manages a libvirt network filter.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Network filter UUID.",
				Computed:    true,
			},

			"name": schema.StringAttribute{
				Description: "Name of the libvirt network filter.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"chain": schema.StringAttribute{
				Description: "Optional libvirt network filter chain.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"priority": schema.Int64Attribute{
				Description: "Optional libvirt network filter priority.",
				Optional:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *NWFilterResource) Configure(
	ctx context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*libvirtclient.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *libvirt.Client, got: %T", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *NWFilterResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var model NWFilterResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	filterXML := libvirtxml.NWFilter{
		Name: model.Name.ValueString(),
	}

	if !model.Chain.IsNull() && !model.Chain.IsUnknown() {
		filterXML.Chain = model.Chain.ValueString()
	}

	if !model.Priority.IsNull() && !model.Priority.IsUnknown() {
		filterXML.Priority = int(model.Priority.ValueInt64())
	}

	xmlDoc, err := filterXML.Marshal()
	if err != nil {
		resp.Diagnostics.AddError(
			"Network Filter XML Generation Failed",
			fmt.Sprintf("Failed to marshal network filter XML: %s", err),
		)
		return
	}

	tflog.Debug(ctx, "Generated network filter XML", map[string]any{
		"xml": xmlDoc,
	})

	filter, err := r.client.Libvirt().NwfilterDefineXML(xmlDoc)
	if err != nil {
		resp.Diagnostics.AddError(
			"Network Filter Creation Failed",
			fmt.Sprintf("Failed to define network filter: %s", err),
		)
		return
	}

	model.ID = types.StringValue(libvirtclient.UUIDString(filter.UUID))

	tflog.Info(ctx, "Created network filter", map[string]any{
		"name": model.Name.ValueString(),
		"uuid": model.ID.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

// Read/refresh will be completed by Person 2.
// Keeping the existing state unchanged is sufficient for Person 1's
// initial Create/Delete implementation.
func (r *NWFilterResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
}

// Iteration 1 currently treats schema changes as replacement.
// All configurable attributes above use RequiresReplace.
func (r *NWFilterResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	resp.Diagnostics.AddError(
		"Network Filter Update Not Supported",
		"Network filter attributes currently require replacement.",
	)
}

func (r *NWFilterResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var model NWFilterResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	filter, err := r.client.Libvirt().NwfilterLookupByName(model.Name.ValueString())
	if err != nil {
		// Treat an already-missing filter as deleted.
		tflog.Warn(ctx, "Network filter was not found during delete", map[string]any{
			"name":  model.Name.ValueString(),
			"error": err.Error(),
		})
		return
	}

	if err := r.client.Libvirt().NwfilterUndefine(filter); err != nil {
		resp.Diagnostics.AddError(
			"Network Filter Delete Failed",
			fmt.Sprintf("Failed to undefine network filter %q: %s",
				model.Name.ValueString(), err),
		)
		return
	}

	tflog.Info(ctx, "Deleted network filter", map[string]any{
		"name": model.Name.ValueString(),
	})
}
