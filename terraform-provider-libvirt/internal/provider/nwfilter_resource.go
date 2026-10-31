package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	libvirt "github.com/digitalocean/go-libvirt"
	libvirtclient "github.com/dmacvicar/terraform-provider-libvirt/v2/internal/libvirt"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
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

func NewNWFilterResource() resource.Resource { return &NWFilterResource{} }

func (r *NWFilterResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nwfilter"
}

func (r *NWFilterResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a libvirt network filter.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Network filter UUID.", Computed: true},
			"name": schema.StringAttribute{
				Description: "Name of the libvirt network filter.", Required: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"chain":    schema.StringAttribute{Description: "Optional libvirt network filter chain.", Optional: true},
			"priority": schema.Int64Attribute{Description: "Optional libvirt network filter priority.", Optional: true},
			"entries": schema.ListNestedAttribute{
				Description: "Ordered entries. Each entry has either rule or ref.", Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"rule": schema.SingleNestedAttribute{
							Description: "Network filter rule.", Optional: true,
							Attributes: map[string]schema.Attribute{
								"action":    schema.StringAttribute{Description: "Rule action.", Required: true},
								"direction": schema.StringAttribute{Description: "Rule direction.", Required: true},
								"priority":  schema.Int64Attribute{Description: "Optional rule priority.", Optional: true},
								"icmp": schema.SingleNestedAttribute{
									Description: "ICMP match criteria.", Required: true,
									Attributes: map[string]schema.Attribute{
										"type": schema.Int64Attribute{Description: "ICMP type.", Required: true},
										"code": schema.Int64Attribute{Description: "ICMP code.", Required: true},
									},
								},
							},
						},
						"ref": schema.SingleNestedAttribute{
							Description: "Reference another network filter.", Optional: true,
							Attributes: map[string]schema.Attribute{
								"filter": schema.StringAttribute{Description: "Referenced filter name.", Required: true},
								"parameters": schema.ListNestedAttribute{
									Description: "Ordered name/value parameters; duplicate names are allowed.", Optional: true,
									NestedObject: schema.NestedAttributeObject{
										Attributes: map[string]schema.Attribute{
											"name":  schema.StringAttribute{Description: "Parameter name.", Required: true},
											"value": schema.StringAttribute{Description: "Parameter value.", Required: true},
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

func (r *NWFilterResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*libvirtclient.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *libvirt.Client, got: %T", req.ProviderData))
		return
	}
	r.client = client
}

func nwFilterXMLFromModel(model NWFilterResourceModel, uuid string) (string, error) {
	filterXML := libvirtxml.NWFilter{Name: model.Name.ValueString(), UUID: uuid}
	if !model.Chain.IsNull() && !model.Chain.IsUnknown() {
		filterXML.Chain = model.Chain.ValueString()
	}
	if !model.Priority.IsNull() && !model.Priority.IsUnknown() {
		filterXML.Priority = int(model.Priority.ValueInt64())
	}
	entries, err := nwFilterEntriesFromModel(model)
	if err != nil {
		return "", err
	}
	filterXML.Entries = entries
	return filterXML.Marshal()
}

func (r *NWFilterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model NWFilterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}
	xmlDoc, err := nwFilterXMLFromModel(model, "")
	if err != nil {
		resp.Diagnostics.AddError("Network Filter XML Generation Failed", err.Error())
		return
	}
	filter, err := r.client.Libvirt().NwfilterDefineXML(xmlDoc)
	if err != nil {
		resp.Diagnostics.AddError("Network Filter Creation Failed", err.Error())
		return
	}
	model.ID = types.StringValue(libvirtclient.UUIDString(filter.UUID))
	tflog.Info(ctx, "Created network filter", map[string]any{"name": model.Name.ValueString(), "uuid": model.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func nwFilterEntriesFromModel(model NWFilterResourceModel) ([]libvirtxml.NWFilterEntry, error) {
	if model.Entries.IsNull() || model.Entries.IsUnknown() {
		return nil, nil
	}
	entries := make([]libvirtxml.NWFilterEntry, 0, len(model.Entries.Elements()))
	for i, rawEntry := range model.Entries.Elements() {
		entry, ok := rawEntry.(types.Object)
		if !ok || entry.IsNull() || entry.IsUnknown() {
			return nil, fmt.Errorf("entry %d is not a valid object", i)
		}
		a := entry.Attributes()
		rule, hasRule := a["rule"].(types.Object)
		hasRule = hasRule && !rule.IsNull() && !rule.IsUnknown()
		ref, hasRef := a["ref"].(types.Object)
		hasRef = hasRef && !ref.IsNull() && !ref.IsUnknown()
		if hasRule == hasRef {
			return nil, fmt.Errorf("entry %d must contain exactly one of rule or ref", i)
		}
		if hasRef {
			refAttrs := ref.Attributes()
			name, ok := refAttrs["filter"].(types.String)
			if !ok || name.IsNull() || name.IsUnknown() || strings.TrimSpace(name.ValueString()) == "" {
				return nil, fmt.Errorf("entry %d has an empty or invalid reference filter name", i)
			}
			if name.ValueString() == model.Name.ValueString() {
				return nil, fmt.Errorf("entry %d cannot reference its own filter", i)
			}
			filterRef := &libvirtxml.NWFilterRef{Filter: name.ValueString()}
			parameters, ok := refAttrs["parameters"].(types.List)
			if !ok {
				return nil, fmt.Errorf("entry %d has invalid reference parameters", i)
			}
			if !parameters.IsNull() && !parameters.IsUnknown() {
				for j, p := range parameters.Elements() {
					pobj, ok := p.(types.Object)
					if !ok || pobj.IsNull() || pobj.IsUnknown() {
						return nil, fmt.Errorf("entry %d parameter %d is invalid", i, j)
					}
					pa := pobj.Attributes()
					pname, ok1 := pa["name"].(types.String)
					pvalue, ok2 := pa["value"].(types.String)
					if !ok1 || pname.IsNull() || pname.IsUnknown() || strings.TrimSpace(pname.ValueString()) == "" {
						return nil, fmt.Errorf("entry %d parameter %d has an invalid name", i, j)
					}
					if !ok2 || pvalue.IsNull() || pvalue.IsUnknown() {
						return nil, fmt.Errorf("entry %d parameter %d has an invalid value", i, j)
					}
					filterRef.Parameters = append(filterRef.Parameters, libvirtxml.NWFilterParameter{
						Name: pname.ValueString(), Value: pvalue.ValueString(),
					})
				}
			}
			entries = append(entries, libvirtxml.NWFilterEntry{Ref: filterRef})
			continue
		}
		ra := rule.Attributes()
		action, ok1 := ra["action"].(types.String)
		direction, ok2 := ra["direction"].(types.String)
		if !ok1 || action.IsNull() || action.IsUnknown() || !ok2 || direction.IsNull() || direction.IsUnknown() {
			return nil, fmt.Errorf("entry %d has invalid rule action or direction", i)
		}
		icmp, ok := ra["icmp"].(types.Object)
		if !ok || icmp.IsNull() || icmp.IsUnknown() {
			return nil, fmt.Errorf("entry %d has invalid ICMP criteria", i)
		}
		ia := icmp.Attributes()
		typ, ok1 := ia["type"].(types.Int64)
		code, ok2 := ia["code"].(types.Int64)
		if !ok1 || typ.IsNull() || typ.IsUnknown() || !ok2 || code.IsNull() || code.IsUnknown() {
			return nil, fmt.Errorf("entry %d has invalid ICMP type or code", i)
		}
		var priority *int64
		if v, ok := ra["priority"].(types.Int64); ok && !v.IsNull() && !v.IsUnknown() {
			val := v.ValueInt64()
			priority = &val
		}
		converted, err := buildICMPNWFilterEntry(action.ValueString(), direction.ValueString(), priority, typ.ValueInt64(), code.ValueInt64())
		if err != nil {
			return nil, fmt.Errorf("entry %d: %w", i, err)
		}
		entries = append(entries, converted)
	}
	return entries, nil
}

func buildICMPNWFilterEntry(action, direction string, priority *int64, icmpType, icmpCode int64) (libvirtxml.NWFilterEntry, error) {
	if icmpType < 0 || icmpType > 255 {
		return libvirtxml.NWFilterEntry{}, fmt.Errorf("ICMP type must be between 0 and 255")
	}
	if icmpCode < 0 || icmpCode > 255 {
		return libvirtxml.NWFilterEntry{}, fmt.Errorf("ICMP code must be between 0 and 255")
	}
	typeValue, codeValue := uint(icmpType), uint(icmpCode)
	rule := &libvirtxml.NWFilterRule{
		Action: action, Direction: direction,
		ICMP: &libvirtxml.NWFilterRuleICMP{
			Type: libvirtxml.NWFilterField{Uint: &typeValue},
			Code: libvirtxml.NWFilterField{Uint: &codeValue},
		},
	}
	if priority != nil {
		rule.Priority = int(*priority)
	}
	return libvirtxml.NWFilterEntry{Rule: rule}, nil
}

// Refresh the configured reference fields from real libvirt XML. Unconfigured
// optional parameter lists stay null to preserve Terraform's configuration intent.
// Rule entries are deliberately left unchanged until full protocol decoding exists.
func refreshNWFilterRefsFromXML(ctx context.Context, original types.List, live []libvirtxml.NWFilterEntry) (types.List, error) {
	if original.IsNull() || original.IsUnknown() {
		return original, nil
	}
	values := original.Elements()
	if len(values) != len(live) {
		return original, nil
	} // Full list drift detection is future work.
	updated := make([]attr.Value, 0, len(values))
	for i, v := range values {
		entryObj, ok := v.(types.Object)
		if !ok {
			return original, fmt.Errorf("entry %d has invalid saved state", i)
		}
		entryAttrs := entryObj.Attributes()
		refObj, ok := entryAttrs["ref"].(types.Object)
		if !ok || refObj.IsNull() || refObj.IsUnknown() || live[i].Ref == nil {
			updated = append(updated, v)
			continue
		}
		refAttrs := refObj.Attributes()
		refAttrs["filter"] = types.StringValue(live[i].Ref.Filter)
		params, ok := refAttrs["parameters"].(types.List)
		if !ok {
			return original, fmt.Errorf("entry %d reference parameters are invalid", i)
		}
		if !params.IsNull() && !params.IsUnknown() {
			paramType := params.ElementType(ctx)
			objType, ok := paramType.(types.ObjectType)
			if !ok {
				return original, fmt.Errorf("entry %d reference parameter type is invalid", i)
			}
			values := make([]attr.Value, 0, len(live[i].Ref.Parameters))
			for _, p := range live[i].Ref.Parameters {
				o, diags := types.ObjectValue(objType.AttrTypes, map[string]attr.Value{
					"name": types.StringValue(p.Name), "value": types.StringValue(p.Value),
				})
				if diags.HasError() {
					return original, fmt.Errorf("entry %d cannot refresh reference parameter: %v", i, diags)
				}
				values = append(values, o)
			}
			list, diags := types.ListValue(paramType, values)
			if diags.HasError() {
				return original, fmt.Errorf("entry %d cannot refresh reference parameters: %v", i, diags)
			}
			refAttrs["parameters"] = list
		}
		refValue, diags := types.ObjectValue(refObj.AttributeTypes(ctx), refAttrs)
		if diags.HasError() {
			return original, fmt.Errorf("entry %d cannot refresh reference: %v", i, diags)
		}
		entryAttrs["ref"] = refValue
		entryValue, diags := types.ObjectValue(entryObj.AttributeTypes(ctx), entryAttrs)
		if diags.HasError() {
			return original, fmt.Errorf("entry %d cannot refresh entry: %v", i, diags)
		}
		updated = append(updated, entryValue)
	}
	list, diags := types.ListValue(original.ElementType(ctx), updated)
	if diags.HasError() {
		return original, fmt.Errorf("cannot refresh entries: %v", diags)
	}
	return list, nil
}

func (r *NWFilterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model NWFilterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}
	filter, err := r.client.Libvirt().NwfilterLookupByName(model.Name.ValueString())
	if err != nil {
		var libvirtErr libvirt.Error
		if errors.As(err, &libvirtErr) && libvirtErr.Code == uint32(libvirt.ErrNoNwfilter) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Network Filter Lookup Failed", fmt.Sprintf("Failed to find network filter %q: %s", model.Name.ValueString(), err))
		return
	}
	xmlDoc, err := r.client.Libvirt().NwfilterGetXMLDesc(filter, 0)
	if err != nil {
		resp.Diagnostics.AddError("Network Filter Read Failed", err.Error())
		return
	}
	var liveFilter libvirtxml.NWFilter
	if err := liveFilter.Unmarshal(xmlDoc); err != nil {
		resp.Diagnostics.AddError("Network Filter XML Parse Failed", err.Error())
		return
	}
	model.ID = types.StringValue(libvirtclient.UUIDString(filter.UUID))
	model.Name = types.StringValue(liveFilter.Name)
	if !model.Chain.IsNull() && !model.Chain.IsUnknown() {
		model.Chain = types.StringValue(liveFilter.Chain)
	}
	if !model.Priority.IsNull() && !model.Priority.IsUnknown() {
		model.Priority = types.Int64Value(int64(liveFilter.Priority))
	}
	model.Entries, err = refreshNWFilterRefsFromXML(ctx, model.Entries, liveFilter.Entries)
	if err != nil {
		resp.Diagnostics.AddError("Network Filter Reference Refresh Failed", err.Error())
		return
	}
	tflog.Debug(ctx, "Refreshed network filter state", map[string]any{"name": model.Name.ValueString(), "uuid": model.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func (r *NWFilterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var old NWFilterResourceModel
	var model NWFilterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &old)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if old.ID.IsNull() || old.ID.IsUnknown() || old.ID.ValueString() == "" {
		resp.Diagnostics.AddError("Invalid Network Filter State", "Cannot update a network filter without its existing UUID.")
		return
	}
	if old.Name.ValueString() != model.Name.ValueString() {
		resp.Diagnostics.AddError("Network Filter Rename Not Supported", "Changing a filter name requires replacement.")
		return
	}
	existing, err := r.client.Libvirt().NwfilterLookupByName(old.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Network Filter Lookup Failed", err.Error())
		return
	}
	if libvirtclient.UUIDString(existing.UUID) != old.ID.ValueString() {
		resp.Diagnostics.AddError("Network Filter Identity Mismatch", "The live UUID differs from Terraform state.")
		return
	}
	xmlDoc, err := nwFilterXMLFromModel(model, old.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Network Filter XML Generation Failed", err.Error())
		return
	}
	updated, err := r.client.Libvirt().NwfilterDefineXML(xmlDoc)
	if err != nil {
		resp.Diagnostics.AddError("Network Filter Update Failed", err.Error())
		return
	}
	if libvirtclient.UUIDString(updated.UUID) != old.ID.ValueString() {
		resp.Diagnostics.AddError("Network Filter UUID Changed", "Libvirt returned a different UUID after update.")
		return
	}
	model.ID = old.ID
	tflog.Info(ctx, "Updated network filter", map[string]any{"name": model.Name.ValueString(), "uuid": model.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func (r *NWFilterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var model NWFilterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}
	filter, err := r.client.Libvirt().NwfilterLookupByName(model.Name.ValueString())
	if err != nil {
		tflog.Warn(ctx, "Network filter was not found during delete", map[string]any{"name": model.Name.ValueString(), "error": err.Error()})
		return
	}
	if err := r.client.Libvirt().NwfilterUndefine(filter); err != nil {
		resp.Diagnostics.AddError("Network Filter Delete Failed", fmt.Sprintf("Failed to undefine filter %q: %s", model.Name.ValueString(), err))
		return
	}
	tflog.Info(ctx, "Deleted network filter", map[string]any{"name": model.Name.ValueString()})
}
