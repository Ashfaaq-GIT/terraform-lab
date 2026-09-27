package provider

import (
	"context"
	"errors"
	"fmt"

	libvirt "github.com/digitalocean/go-libvirt"

	libvirtclient "github.com/dmacvicar/terraform-provider-libvirt/v2/internal/libvirt"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
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
	Entries  types.List   `tfsdk:"entries"`
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

			"entries": schema.ListNestedAttribute{
				Description: "Ordered network filter entries. Iteration 1 supports ICMP rules.",
				Optional:    true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"rule": schema.SingleNestedAttribute{
							Description: "Network filter rule.",
							Required:    true,
							Attributes: map[string]schema.Attribute{
								"action": schema.StringAttribute{
									Description: "Rule action, such as accept or drop.",
									Required:    true,
								},
								"direction": schema.StringAttribute{
									Description: "Rule direction, such as in or out.",
									Required:    true,
								},
								"priority": schema.Int64Attribute{
									Description: "Optional rule priority.",
									Optional:    true,
								},
								"icmp": schema.SingleNestedAttribute{
									Description: "ICMP match criteria.",
									Required:    true,
									Attributes: map[string]schema.Attribute{
										"type": schema.Int64Attribute{
											Description: "ICMP type.",
											Required:    true,
										},
										"code": schema.Int64Attribute{
											Description: "ICMP code.",
											Required:    true,
										},
									},
								},
							},
						},
					},
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

	entries, err := nwFilterEntriesFromModel(model)
	if err != nil {
		resp.Diagnostics.AddError(
			"Network Filter Entry Conversion Failed",
			fmt.Sprintf("Failed to convert network filter entries: %s", err),
		)
		return
	}

	filterXML.Entries = entries

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
func nwFilterEntriesFromModel(model NWFilterResourceModel) ([]libvirtxml.NWFilterEntry, error) {
	if model.Entries.IsNull() || model.Entries.IsUnknown() {
		return nil, nil
	}

	entries := make([]libvirtxml.NWFilterEntry, 0, len(model.Entries.Elements()))

	for i, entryValue := range model.Entries.Elements() {
		entryObject, ok := entryValue.(types.Object)
		if !ok {
			return nil, fmt.Errorf("entry %d is not an object", i)
		}

		ruleValue, ok := entryObject.Attributes()["rule"]
		if !ok {
			return nil, fmt.Errorf("entry %d does not contain a rule", i)
		}

		ruleObject, ok := ruleValue.(types.Object)
		if !ok || ruleObject.IsNull() || ruleObject.IsUnknown() {
			return nil, fmt.Errorf("entry %d has an invalid rule", i)
		}

		ruleAttrs := ruleObject.Attributes()

		actionValue, ok := ruleAttrs["action"].(types.String)
		if !ok || actionValue.IsNull() || actionValue.IsUnknown() {
			return nil, fmt.Errorf("entry %d rule has an invalid action", i)
		}

		directionValue, ok := ruleAttrs["direction"].(types.String)
		if !ok || directionValue.IsNull() || directionValue.IsUnknown() {
			return nil, fmt.Errorf("entry %d rule has an invalid direction", i)
		}

		icmpValue, ok := ruleAttrs["icmp"]
		if !ok {
			return nil, fmt.Errorf("entry %d rule does not contain ICMP criteria", i)
		}

		icmpObject, ok := icmpValue.(types.Object)
		if !ok || icmpObject.IsNull() || icmpObject.IsUnknown() {
			return nil, fmt.Errorf("entry %d rule has invalid ICMP criteria", i)
		}

		icmpAttrs := icmpObject.Attributes()

		typeValue, ok := icmpAttrs["type"].(types.Int64)
		if !ok || typeValue.IsNull() || typeValue.IsUnknown() {
			return nil, fmt.Errorf("entry %d ICMP type is invalid", i)
		}

		codeValue, ok := icmpAttrs["code"].(types.Int64)
		if !ok || codeValue.IsNull() || codeValue.IsUnknown() {
			return nil, fmt.Errorf("entry %d ICMP code is invalid", i)
		}

		var priority *int64
		if priorityValue, ok := ruleAttrs["priority"].(types.Int64); ok &&
			!priorityValue.IsNull() &&
			!priorityValue.IsUnknown() {
			value := priorityValue.ValueInt64()
			priority = &value
		}

		entry, err := buildICMPNWFilterEntry(
			actionValue.ValueString(),
			directionValue.ValueString(),
			priority,
			typeValue.ValueInt64(),
			codeValue.ValueInt64(),
		)
		if err != nil {
			return nil, fmt.Errorf("entry %d: %w", i, err)
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

func buildICMPNWFilterEntry(
	action string,
	direction string,
	priority *int64,
	icmpType int64,
	icmpCode int64,
) (libvirtxml.NWFilterEntry, error) {
	if icmpType < 0 || icmpType > 255 {
		return libvirtxml.NWFilterEntry{}, fmt.Errorf(
			"ICMP type must be between 0 and 255",
		)
	}

	if icmpCode < 0 || icmpCode > 255 {
		return libvirtxml.NWFilterEntry{}, fmt.Errorf(
			"ICMP code must be between 0 and 255",
		)
	}

	typeValue := uint(icmpType)
	codeValue := uint(icmpCode)

	rule := &libvirtxml.NWFilterRule{
		Action:    action,
		Direction: direction,
		ICMP: &libvirtxml.NWFilterRuleICMP{
			Type: libvirtxml.NWFilterField{
				Uint: &typeValue,
			},
			Code: libvirtxml.NWFilterField{
				Uint: &codeValue,
			},
		},
	}

	if priority != nil {
		rule.Priority = int(*priority)
	}

	return libvirtxml.NWFilterEntry{
		Rule: rule,
	}, nil
}

func (r *NWFilterResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var model NWFilterResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Look up the live network filter by its Terraform name.
	filter, err := r.client.Libvirt().NwfilterLookupByName(model.Name.ValueString())
	if err != nil {
		var libvirtErr libvirt.Error

		// If the filter was deleted outside Terraform, remove it from state.
		if errors.As(err, &libvirtErr) &&
			libvirtErr.Code == uint32(libvirt.ErrNoNwfilter) {

			tflog.Warn(ctx, "Network filter no longer exists", map[string]any{
				"name": model.Name.ValueString(),
			})

			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Network Filter Lookup Failed",
			fmt.Sprintf(
				"Failed to find network filter %q: %s",
				model.Name.ValueString(),
				err,
			),
		)
		return
	}

	// Read the live XML from libvirt.
	xmlDoc, err := r.client.Libvirt().NwfilterGetXMLDesc(filter, 0)
	if err != nil {
		resp.Diagnostics.AddError(
			"Network Filter Read Failed",
			fmt.Sprintf(
				"Failed to read network filter %q XML: %s",
				model.Name.ValueString(),
				err,
			),
		)
		return
	}

	var liveFilter libvirtxml.NWFilter

	if err := liveFilter.Unmarshal(xmlDoc); err != nil {
		resp.Diagnostics.AddError(
			"Network Filter XML Parse Failed",
			fmt.Sprintf(
				"Failed to parse network filter %q XML: %s",
				model.Name.ValueString(),
				err,
			),
		)
		return
	}

	// Refresh identity and live values.
	model.ID = types.StringValue(libvirtclient.UUIDString(filter.UUID))
	model.Name = types.StringValue(liveFilter.Name)

	// Preserve optional attributes when the user did not configure them.
	// If configured, refresh them from live libvirt state so drift can be seen.
	if !model.Chain.IsNull() && !model.Chain.IsUnknown() {
		model.Chain = types.StringValue(liveFilter.Chain)
	}

	if !model.Priority.IsNull() && !model.Priority.IsUnknown() {
		model.Priority = types.Int64Value(int64(liveFilter.Priority))
	}

	tflog.Debug(ctx, "Refreshed network filter state", map[string]any{
		"name": model.Name.ValueString(),
		"uuid": model.ID.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
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
