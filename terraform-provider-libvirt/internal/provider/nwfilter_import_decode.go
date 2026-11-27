package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"libvirt.org/go/libvirtxml"
)

func importNullAttributes(t types.ObjectType) map[string]attr.Value {
	a := make(map[string]attr.Value)
	for name, typ := range t.AttrTypes {
		switch v := typ.(type) {
		case basetypes.StringType:
			a[name] = types.StringNull()
		case basetypes.Int64Type:
			a[name] = types.Int64Null()
		case types.ObjectType:
			a[name] = types.ObjectNull(v.AttrTypes)
		case types.ListType:
			a[name] = types.ListNull(v.ElemType)
		}
	}
	return a
}

func importProtocolFields(
	t types.ObjectType,
	text map[string]libvirtxml.NWFilterField,
	numbers map[string]libvirtxml.NWFilterField,
) (types.Object, error) {
	a := importNullAttributes(t)

	for name, f := range text {
		if f.Var != "" || f.Uint != nil {
			return types.Object{}, fmt.Errorf(
				"unsupported variable or numeric value for %s", name)
		}
		if f.Str != "" {
			a[name] = types.StringValue(f.Str)
		}
	}
	for name, f := range numbers {
		if f.Var != "" {
			return types.Object{}, fmt.Errorf(
				"unsupported variable for %s", name)
		}

		if f.Uint != nil {
			a[name] = types.Int64Value(int64(*f.Uint))
		} else if f.Str != "" {
			n, err := strconv.ParseUint(f.Str, 10, 16)
			if err != nil {
				return types.Object{}, fmt.Errorf(
					"invalid decimal number for %s: %q", name, f.Str)
			}
			a[name] = types.Int64Value(int64(n))
		}
	}

	obj, diags := types.ObjectValue(t.AttrTypes, a)
	if diags.HasError() {
		return types.Object{}, fmt.Errorf("protocol conversion: %v", diags)
	}
	return obj, nil
}

func importNWFilterEntries(ctx context.Context, live []libvirtxml.NWFilterEntry) (types.List, error) {
	var schemaResp resource.SchemaResponse
	(&NWFilterResource{}).Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	listType := schemaResp.Schema.Attributes["entries"].GetType().(types.ListType)
	entryType := listType.ElemType.(types.ObjectType)
	ruleType := entryType.AttrTypes["rule"].(types.ObjectType)
	refType := entryType.AttrTypes["ref"].(types.ObjectType)

	entries := make([]attr.Value, 0, len(live))

	for i, entry := range live {
		var ruleValue attr.Value = types.ObjectNull(ruleType.AttrTypes)
		var refValue attr.Value = types.ObjectNull(refType.AttrTypes)

		if entry.Ref != nil && entry.Rule == nil {
			refAttrs := importNullAttributes(refType)
			refAttrs["filter"] = types.StringValue(entry.Ref.Filter)

			paramType := refType.AttrTypes["parameters"].(types.ListType).ElemType.(types.ObjectType)
			if entry.Ref.Parameters != nil {
				params := make([]attr.Value, 0, len(entry.Ref.Parameters))
				for _, p := range entry.Ref.Parameters {
					obj, diags := types.ObjectValue(paramType.AttrTypes, map[string]attr.Value{
						"name":  types.StringValue(p.Name),
						"value": types.StringValue(p.Value),
					})
					if diags.HasError() {
						return types.ListNull(entryType), fmt.Errorf("entry %d parameter: %v", i, diags)
					}
					params = append(params, obj)
				}
				list, diags := types.ListValue(paramType, params)
				if diags.HasError() {
					return types.ListNull(entryType), fmt.Errorf("entry %d parameters: %v", i, diags)
				}
				refAttrs["parameters"] = list
			}

			obj, diags := types.ObjectValue(refType.AttrTypes, refAttrs)
			if diags.HasError() {
				return types.ListNull(entryType), fmt.Errorf("entry %d reference: %v", i, diags)
			}
			refValue = obj

		} else if entry.Rule != nil && entry.Ref == nil {
			r := entry.Rule
			ruleAttrs := importNullAttributes(ruleType)
			ruleAttrs["action"] = types.StringValue(r.Action)
			ruleAttrs["direction"] = types.StringValue(r.Direction)
			// Libvirt assigns priority 500 when Terraform omits it.
			// Normalize that default to null for import verification.
			if r.Priority != 0 && r.Priority != 500 {
				ruleAttrs["priority"] = types.Int64Value(int64(r.Priority))
			}

			var protocol string
			var textFields map[string]libvirtxml.NWFilterField
			var numberFields map[string]libvirtxml.NWFilterField

			switch {
			case r.ICMP != nil:
				protocol = "icmp"
				numberFields = map[string]libvirtxml.NWFilterField{
					"type": r.ICMP.Type, "code": r.ICMP.Code,
				}
				if r.ICMP.Type.Uint == nil || r.ICMP.Code.Uint == nil {
					return types.ListNull(entryType), fmt.Errorf(
						"entry %d requires numeric ICMP type and code", i)
				}

			case r.TCP != nil || r.UDP != nil || r.IP != nil:
				var src, srcMask, dst, dstMask libvirtxml.NWFilterField
				var ports libvirtxml.NWFilterRuleCommonPort
				var ipProtocol libvirtxml.NWFilterField

				switch {
				case r.TCP != nil:
					protocol = "tcp"
					src = r.TCP.SrcIPAddr
					srcMask = r.TCP.SrcIPMask
					dst = r.TCP.DstIPAddr
					dstMask = r.TCP.DstIPMask
					ports = r.TCP.NWFilterRuleCommonPort
				case r.UDP != nil:
					protocol = "udp"
					src = r.UDP.SrcIPAddr
					srcMask = r.UDP.SrcIPMask
					dst = r.UDP.DstIPAddr
					dstMask = r.UDP.DstIPMask
					ports = r.UDP.NWFilterRuleCommonPort
				case r.IP != nil:
					protocol = "ip"
					src = r.IP.SrcIPAddr
					srcMask = r.IP.SrcIPMask
					dst = r.IP.DstIPAddr
					dstMask = r.IP.DstIPMask
					ports = r.IP.NWFilterRuleCommonPort
					ipProtocol = r.IP.Protocol
				}

				textFields = map[string]libvirtxml.NWFilterField{
					"src_ip": src, "src_ip_mask": srcMask,
					"dst_ip": dst, "dst_ip_mask": dstMask,
				}
				if protocol == "ip" {
					textFields["protocol"] = ipProtocol
				}
				numberFields = map[string]libvirtxml.NWFilterField{
					"src_port_start": ports.SrcPortStart,
					"src_port_end":   ports.SrcPortEnd,
					"dst_port_start": ports.DstPortStart,
					"dst_port_end":   ports.DstPortEnd,
				}

			case r.MAC != nil:
				protocol = "mac"
				textFields = map[string]libvirtxml.NWFilterField{
					"src_mac":      r.MAC.SrcMACAddr,
					"src_mac_mask": r.MAC.SrcMACMask,
					"dst_mac":      r.MAC.DstMACAddr,
					"dst_mac_mask": r.MAC.DstMACMask,
					"protocol_id":  r.MAC.ProtocolID,
				}

			default:
				return types.ListNull(entryType), fmt.Errorf(
					"entry %d: unsupported rule type during import", i)
			}

			protoType := ruleType.AttrTypes[protocol].(types.ObjectType)
			protoObj, err := importProtocolFields(protoType, textFields, numberFields)
			if err != nil {
				return types.ListNull(entryType), fmt.Errorf("entry %d: %w", i, err)
			}
			ruleAttrs[protocol] = protoObj

			obj, diags := types.ObjectValue(ruleType.AttrTypes, ruleAttrs)
			if diags.HasError() {
				return types.ListNull(entryType), fmt.Errorf("entry %d rule: %v", i, diags)
			}
			ruleValue = obj

		} else {
			return types.ListNull(entryType), fmt.Errorf(
				"entry %d must contain exactly one rule or reference", i)
		}

		obj, diags := types.ObjectValue(entryType.AttrTypes, map[string]attr.Value{
			"rule": ruleValue,
			"ref":  refValue,
		})
		if diags.HasError() {
			return types.ListNull(entryType), fmt.Errorf("entry %d: %v", i, diags)
		}
		entries = append(entries, obj)
	}

	result, diags := types.ListValue(entryType, entries)
	if diags.HasError() {
		return types.ListNull(entryType), fmt.Errorf("entries: %v", diags)
	}
	return result, nil
}
