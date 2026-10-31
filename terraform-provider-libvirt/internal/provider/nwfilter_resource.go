package provider

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
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
									Description: "ICMP match criteria.", Optional: true,
									Attributes: map[string]schema.Attribute{
										"type": schema.Int64Attribute{Description: "ICMP type.", Required: true},
										"code": schema.Int64Attribute{Description: "ICMP code.", Required: true},
									},
								},
								"tcp": schema.SingleNestedAttribute{
									Description: "IPv4 TCP criteria including address and port ranges.", Optional: true,
									Attributes: nwFilterIPv4Attributes(true, false),
								},
								"udp": schema.SingleNestedAttribute{
									Description: "IPv4 UDP criteria including address and port ranges.", Optional: true,
									Attributes: nwFilterIPv4Attributes(true, false),
								},
								"ip": schema.SingleNestedAttribute{
									Description: "IPv4 packet criteria including optional layer-4 protocol and ports.", Optional: true,
									Attributes: nwFilterIPv4Attributes(true, true),
								},
								"mac": schema.SingleNestedAttribute{
									Description: "Ethernet source/destination MAC criteria.", Optional: true,
									Attributes: map[string]schema.Attribute{
										"src_mac":      schema.StringAttribute{Optional: true, Description: "Source MAC address."},
										"src_mac_mask": schema.StringAttribute{Optional: true, Description: "Source MAC mask."},
										"dst_mac":      schema.StringAttribute{Optional: true, Description: "Destination MAC address."},
										"dst_mac_mask": schema.StringAttribute{Optional: true, Description: "Destination MAC mask."},
										"protocol_id":  schema.StringAttribute{Optional: true, Description: "Optional Ethernet protocol identifier: ipv4, ipv6, arp or rarp."},
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

// The same IPv4 fields apply to TCP, UDP and IPv4 rules; protocol is only
// exposed on <ip>. Each call returns independent schema maps.
func nwFilterIPv4Attributes(ports, protocol bool) map[string]schema.Attribute {
	fields := map[string]schema.Attribute{
		"src_ip":      schema.StringAttribute{Optional: true, Description: "Source IPv4 address."},
		"src_ip_mask": schema.StringAttribute{Optional: true, Description: "Source IPv4 mask (prefix 0-32 or dotted decimal)."},
		"dst_ip":      schema.StringAttribute{Optional: true, Description: "Destination IPv4 address."},
		"dst_ip_mask": schema.StringAttribute{Optional: true, Description: "Destination IPv4 mask (prefix 0-32 or dotted decimal)."},
	}
	if ports {
		fields["src_port_start"] = schema.Int64Attribute{Optional: true, Description: "Source port range start (0-65535)."}
		fields["src_port_end"] = schema.Int64Attribute{Optional: true, Description: "Source port range end (0-65535)."}
		fields["dst_port_start"] = schema.Int64Attribute{Optional: true, Description: "Destination port range start (0-65535)."}
		fields["dst_port_end"] = schema.Int64Attribute{Optional: true, Description: "Destination port range end (0-65535)."}
	}
	if protocol {
		fields["protocol"] = schema.StringAttribute{Optional: true, Description: "Layer-4 protocol: tcp, udp, udplite, esp, ah, icmp, igmp or sctp."}
	}
	return fields
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
		converted, err := buildNWFilterProtocolEntry(ra)
		if err != nil {
			return nil, fmt.Errorf("entry %d: %w", i, err)
		}
		entries = append(entries, converted)
	}
	return entries, nil
}

// buildNWFilterProtocolEntry chooses exactly one protocol; this preserves the
// old ICMP path while adding TCP, UDP, IPv4 and Ethernet MAC criteria.
func buildNWFilterProtocolEntry(a map[string]attr.Value) (libvirtxml.NWFilterEntry, error) {
	action, ok := a["action"].(types.String)
	if !ok || action.IsNull() || action.IsUnknown() || strings.TrimSpace(action.ValueString()) == "" {
		return libvirtxml.NWFilterEntry{}, fmt.Errorf("rule action is required")
	}
	direction, ok := a["direction"].(types.String)
	if !ok || direction.IsNull() || direction.IsUnknown() || strings.TrimSpace(direction.ValueString()) == "" {
		return libvirtxml.NWFilterEntry{}, fmt.Errorf("rule direction is required")
	}
	var priority *int64
	if v, ok := a["priority"].(types.Int64); ok && !v.IsNull() && !v.IsUnknown() {
		value := v.ValueInt64()
		priority = &value
	}
	var selected string
	var criteria types.Object
	for _, name := range []string{"icmp", "tcp", "udp", "ip", "mac"} {
		raw, found := a[name]
		if !found {
			continue
		} // permits older unit-test object shapes
		object, ok := raw.(types.Object)
		if !ok || object.IsUnknown() {
			return libvirtxml.NWFilterEntry{}, fmt.Errorf("invalid %s criteria", name)
		}
		if object.IsNull() {
			continue
		}
		if selected != "" {
			return libvirtxml.NWFilterEntry{}, fmt.Errorf("rule must configure exactly one protocol block (icmp, tcp, udp, ip or mac)")
		}
		selected, criteria = name, object
	}
	if selected == "" {
		return libvirtxml.NWFilterEntry{}, fmt.Errorf("rule must configure exactly one protocol block (icmp, tcp, udp, ip or mac)")
	}
	attrs := criteria.Attributes()
	if selected == "icmp" {
		typ, ok1 := attrs["type"].(types.Int64)
		code, ok2 := attrs["code"].(types.Int64)
		if !ok1 || typ.IsNull() || typ.IsUnknown() || !ok2 || code.IsNull() || code.IsUnknown() {
			return libvirtxml.NWFilterEntry{}, fmt.Errorf("invalid ICMP type or code")
		}
		return buildICMPNWFilterEntry(action.ValueString(), direction.ValueString(), priority, typ.ValueInt64(), code.ValueInt64())
	}
	rule := &libvirtxml.NWFilterRule{Action: action.ValueString(), Direction: direction.ValueString()}
	if priority != nil {
		rule.Priority = int(*priority)
	}
	switch selected {
	case "tcp", "udp", "ip":
		common, err := parseNWFilterIPv4(attrs)
		if err != nil {
			return libvirtxml.NWFilterEntry{}, err
		}
		ports, hasPorts, err := parseNWFilterPorts(attrs)
		if err != nil {
			return libvirtxml.NWFilterEntry{}, err
		}
		switch selected {
		case "tcp":
			rule.TCP = &libvirtxml.NWFilterRuleTCP{NWFilterRuleCommonIP: common, NWFilterRuleCommonPort: ports}
		case "udp":
			rule.UDP = &libvirtxml.NWFilterRuleUDP{NWFilterRuleCommonIP: common, NWFilterRuleCommonPort: ports}
		case "ip":
			protocol, present, err := nwFilterOptionalString(attrs, "protocol")
			if err != nil {
				return libvirtxml.NWFilterEntry{}, err
			}
			if present {
				if !nwFilterAllowed(protocol, "tcp", "udp", "udplite", "esp", "ah", "icmp", "igmp", "sctp") {
					return libvirtxml.NWFilterEntry{}, fmt.Errorf("invalid IPv4 protocol %q", protocol)
				}
			}
			if hasPorts && (!present || !nwFilterAllowed(protocol, "tcp", "udp", "sctp")) {
				return libvirtxml.NWFilterEntry{}, fmt.Errorf("IPv4 port criteria require protocol tcp, udp or sctp")
			}
			ipRule := &libvirtxml.NWFilterRuleIP{
				SrcIPAddr: common.SrcIPAddr, SrcIPMask: common.SrcIPMask,
				DstIPAddr: common.DstIPAddr, DstIPMask: common.DstIPMask,
				NWFilterRuleCommonPort: ports,
			}
			if present {
				ipRule.Protocol = libvirtxml.NWFilterField{Str: protocol}
			}
			rule.IP = ipRule
		}
	case "mac":
		mac, err := parseNWFilterMAC(attrs)
		if err != nil {
			return libvirtxml.NWFilterEntry{}, err
		}
		rule.MAC = &mac
	}
	return libvirtxml.NWFilterEntry{Rule: rule}, nil
}

func nwFilterAllowed(value string, options ...string) bool {
	for _, option := range options {
		if value == option {
			return true
		}
	}
	return false
}

func nwFilterOptionalString(a map[string]attr.Value, name string) (string, bool, error) {
	raw, exists := a[name]
	if !exists {
		return "", false, nil
	}
	v, ok := raw.(types.String)
	if !ok || v.IsUnknown() {
		return "", false, fmt.Errorf("%s must be a known string", name)
	}
	if v.IsNull() {
		return "", false, nil
	}
	value := strings.TrimSpace(v.ValueString())
	if value == "" {
		return "", false, fmt.Errorf("%s cannot be blank", name)
	}
	return value, true, nil
}

func nwFilterOptionalPort(a map[string]attr.Value, name string) (uint, bool, error) {
	raw, exists := a[name]
	if !exists {
		return 0, false, nil
	}
	v, ok := raw.(types.Int64)
	if !ok || v.IsUnknown() {
		return 0, false, fmt.Errorf("%s must be a known integer", name)
	}
	if v.IsNull() {
		return 0, false, nil
	}
	n := v.ValueInt64()
	if n < 0 || n > 65535 {
		return 0, false, fmt.Errorf("%s must be between 0 and 65535", name)
	}
	return uint(n), true, nil
}

func parseNWFilterPorts(a map[string]attr.Value) (libvirtxml.NWFilterRuleCommonPort, bool, error) {
	var p libvirtxml.NWFilterRuleCommonPort
	var any bool
	for _, side := range []string{"src", "dst"} {
		start, hasStart, err := nwFilterOptionalPort(a, side+"_port_start")
		if err != nil {
			return p, false, err
		}
		end, hasEnd, err := nwFilterOptionalPort(a, side+"_port_end")
		if err != nil {
			return p, false, err
		}
		if hasEnd && !hasStart {
			return p, false, fmt.Errorf("%s_port_end requires %s_port_start", side, side)
		}
		if hasStart && hasEnd && end < start {
			return p, false, fmt.Errorf("%s port range end cannot be before start", side)
		}
		if hasStart {
			any = true
		}
		if side == "src" {
			if hasStart {
				p.SrcPortStart = libvirtxml.NWFilterField{Uint: &start}
			}
			if hasEnd {
				p.SrcPortEnd = libvirtxml.NWFilterField{Uint: &end}
			}
		} else {
			if hasStart {
				p.DstPortStart = libvirtxml.NWFilterField{Uint: &start}
			}
			if hasEnd {
				p.DstPortEnd = libvirtxml.NWFilterField{Uint: &end}
			}
		}
	}
	return p, any, nil
}

func validateNWFilterMask(value string) error {
	if n, err := strconv.Atoi(value); err == nil {
		if n >= 0 && n <= 32 {
			return nil
		}
		return fmt.Errorf("IPv4 mask prefix must be between 0 and 32")
	}
	ip := net.ParseIP(value).To4()
	if ip == nil {
		return fmt.Errorf("invalid IPv4 mask %q", value)
	}
	ones, bits := net.IPMask(ip).Size()
	if bits != 32 || ones < 0 {
		return fmt.Errorf("IPv4 mask %q is not contiguous", value)
	}
	return nil
}

func parseNWFilterIPv4(a map[string]attr.Value) (libvirtxml.NWFilterRuleCommonIP, error) {
	var ip libvirtxml.NWFilterRuleCommonIP
	for _, side := range []string{"src", "dst"} {
		address, hasAddress, err := nwFilterOptionalString(a, side+"_ip")
		if err != nil {
			return ip, err
		}
		mask, hasMask, err := nwFilterOptionalString(a, side+"_ip_mask")
		if err != nil {
			return ip, err
		}
		if hasAddress && net.ParseIP(address).To4() == nil {
			return ip, fmt.Errorf("invalid %s IPv4 address %q", side, address)
		}
		if hasMask && !hasAddress {
			return ip, fmt.Errorf("%s_ip_mask requires %s_ip", side, side)
		}
		if hasMask {
			if err := validateNWFilterMask(mask); err != nil {
				return ip, fmt.Errorf("%s_ip_mask: %w", side, err)
			}
		}
		if side == "src" {
			if hasAddress {
				ip.SrcIPAddr = libvirtxml.NWFilterField{Str: address}
			}
			if hasMask {
				ip.SrcIPMask = libvirtxml.NWFilterField{Str: mask}
			}
		} else {
			if hasAddress {
				ip.DstIPAddr = libvirtxml.NWFilterField{Str: address}
			}
			if hasMask {
				ip.DstIPMask = libvirtxml.NWFilterField{Str: mask}
			}
		}
	}
	return ip, nil
}

var nwFilterMACVariable = regexp.MustCompile(`^\$[A-Za-z_][A-Za-z0-9_]*$`)

func validateNWFilterMAC(value string) error {
	if nwFilterMACVariable.MatchString(value) {
		return nil
	} // libvirt parameter substitution (e.g. $MAC)
	addr, err := net.ParseMAC(value)
	if err != nil || len(addr) != 6 {
		return fmt.Errorf("invalid Ethernet MAC address %q", value)
	}
	return nil
}

func nwFilterMACField(value string) libvirtxml.NWFilterField {
	if nwFilterMACVariable.MatchString(value) {
		return libvirtxml.NWFilterField{Str: value}
	}
	return libvirtxml.NWFilterField{Str: value}
}

func parseNWFilterMAC(a map[string]attr.Value) (libvirtxml.NWFilterRuleMAC, error) {
	var mac libvirtxml.NWFilterRuleMAC
	any := false
	for _, side := range []string{"src", "dst"} {
		address, hasAddress, err := nwFilterOptionalString(a, side+"_mac")
		if err != nil {
			return mac, err
		}
		mask, hasMask, err := nwFilterOptionalString(a, side+"_mac_mask")
		if err != nil {
			return mac, err
		}
		if hasMask && !hasAddress {
			return mac, fmt.Errorf("%s_mac_mask requires %s_mac", side, side)
		}
		if hasAddress {
			if err := validateNWFilterMAC(address); err != nil {
				return mac, err
			}
			any = true
		}
		if hasMask {
			if err := validateNWFilterMAC(mask); err != nil {
				return mac, err
			}
		}
		if side == "src" {
			if hasAddress {
				mac.SrcMACAddr = nwFilterMACField(address)
			}
			if hasMask {
				mac.SrcMACMask = nwFilterMACField(mask)
			}
		} else {
			if hasAddress {
				mac.DstMACAddr = nwFilterMACField(address)
			}
			if hasMask {
				mac.DstMACMask = nwFilterMACField(mask)
			}
		}
	}
	protocolID, present, err := nwFilterOptionalString(a, "protocol_id")
	if err != nil {
		return mac, err
	}
	if present {
		if !nwFilterAllowed(protocolID, "arp", "rarp", "ipv4", "ipv6") {
			return mac, fmt.Errorf("invalid MAC protocol_id %q", protocolID)
		}
		mac.ProtocolID = libvirtxml.NWFilterField{Str: protocolID}
		any = true
	}
	if !any {
		return mac, fmt.Errorf("MAC rule requires an address or protocol_id")
	}
	return mac, nil
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
