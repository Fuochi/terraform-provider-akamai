package property

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v2/pkg/papi"
	"github.com/akamai/terraform-provider-akamai/v2/pkg/akamai"
	"github.com/akamai/terraform-provider-akamai/v2/pkg/tools"
)

func resourceSecureEdgeHostName() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceSecureEdgeHostNameCreate,
		ReadContext:   resourceSecureEdgeHostNameRead,
		DeleteContext: resourceSecureEdgeHostNameDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourceSecureEdgeHostNameImport,
		},
		Schema: akamaiSecureEdgeHostNameSchema,
	}
}

var akamaiSecureEdgeHostNameSchema = map[string]*schema.Schema{
	"product": {
		Type:     schema.TypeString,
		Required: true,
		ForceNew: true,
	},
	"contract": {
		Type:     schema.TypeString,
		Required: true,
		ForceNew: true,
	},
	"group": {
		Type:     schema.TypeString,
		Required: true,
		ForceNew: true,
	},
	"edge_hostname": {
		Type:     schema.TypeString,
		Required: true,
		ForceNew: true,
	},
	"ipv4": {
		Type:     schema.TypeBool,
		Optional: true,
		Default:  true,
		ForceNew: true,
	},
	"ipv6": {
		Type:     schema.TypeBool,
		Optional: true,
		Default:  false,
		ForceNew: true,
	},
	"ip_behavior": {
		Type:     schema.TypeString,
		Computed: true,
	},
	"certificate": {
		Type:     schema.TypeInt,
		Optional: true,
		ForceNew: true,
	},
}

func resourceSecureEdgeHostNameCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	meta := akamai.Meta(m)
	logger := meta.Log("PAPI", "resourceSecureEdgeHostNameCreate")

	client := inst.Client(meta)

	group, err := getGroup(ctx, d, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	contract, err := getContract(ctx, d, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	logger.Debugf("  Edgehostnames GROUP = %v", group)
	logger.Debugf("Edgehostnames CONTRACT = %v", contract)

	product, err := getProduct(ctx, d, contract.ContractID, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	edgeHostnames, err := client.GetEdgeHostnames(ctx, papi.GetEdgeHostnamesRequest{
		ContractID: contract.ContractID,
		GroupID:    group.GroupID,
	})
	if err != nil {
		return diag.FromErr(err)
	}
	edgeHostname, err := tools.GetStringValue("edge_hostname", d)
	if err != nil {
		return diag.FromErr(err)
	}
	newHostname := papi.EdgeHostnameCreate{}
	newHostname.ProductID = product.ProductID

	switch {
	case strings.HasSuffix(edgeHostname, ".edgesuite.net"):
		newHostname.DomainPrefix = strings.TrimSuffix(edgeHostname, ".edgesuite.net")
		newHostname.DomainSuffix = "edgesuite.net"
		newHostname.SecureNetwork = "STANDARD_TLS"
	case strings.HasSuffix(edgeHostname, ".edgekey.net"):
		newHostname.DomainPrefix = strings.TrimSuffix(edgeHostname, ".edgekey.net")
		newHostname.DomainSuffix = "edgekey.net"
		newHostname.SecureNetwork = "ENHANCED_TLS"
	case strings.HasSuffix(edgeHostname, ".akamaized.net"):
		newHostname.DomainPrefix = strings.TrimSuffix(edgeHostname, ".akamaized.net")
		newHostname.DomainSuffix = "akamaized.net"
		newHostname.SecureNetwork = "SHARED_CERT"
	}

	if newHostname.DomainSuffix == "" && edgeHostname != "" {
		newHostname.DomainSuffix = "edgesuite.net"
		if strings.HasSuffix(edgeHostname, "edgekey.net") {
			newHostname.DomainSuffix = "edgekey.net"
		}
	}

	if newHostname.DomainPrefix == "" && edgeHostname != "" {
		newHostname.DomainPrefix = strings.TrimSuffix(edgeHostname, "."+newHostname.DomainSuffix)
	}

	for _, h := range edgeHostnames.EdgeHostnames.Items {
		if h.DomainPrefix == newHostname.DomainPrefix && h.DomainSuffix == newHostname.DomainSuffix {
			d.SetId(h.ID)
			return nil
		}
	}

	ipv4, _ := tools.GetBoolValue("ipv4", d)
	if ipv4 {
		newHostname.IPVersionBehavior = "IPV4"
	}
	ipv6, _ := tools.GetBoolValue("ipv6", d)
	if ipv6 {
		newHostname.IPVersionBehavior = "IPV6"
	}
	if ipv4 && ipv6 {
		newHostname.IPVersionBehavior = "IPV6_COMPLIANCE"
	}
	if !(ipv4 || ipv6) {
		return diag.FromErr(fmt.Errorf("ipv4 or ipv6 must be specified to create a new Edge Hostname"))
	}

	if err := d.Set("ip_behavior", newHostname.IPVersionBehavior); err != nil {
		return diag.FromErr(fmt.Errorf("%w: %s", tools.ErrValueSet, err.Error()))
	}

	certEnrollmentID, err := tools.GetIntValue("certificate", d)
	if err != nil {
		if !errors.Is(err, tools.ErrNotFound) {
			return diag.FromErr(err)
		}
		if newHostname.SecureNetwork == "ENHANCED_TLS" {
			return diag.FromErr(errors.New("A certificate enrollment ID is required for Enhanced TLS (edgekey.net) edge hostnames"))
		}
	}

	newHostname.CertEnrollmentID = certEnrollmentID
	newHostname.SlotNumber = certEnrollmentID

	logger.Debugf("Creating new edge hostname: %#v", newHostname)
	hostname, err := client.CreateEdgeHostname(ctx, papi.CreateEdgeHostnameRequest{
		EdgeHostname: newHostname,
		ContractID:   contract.ContractID,
		GroupID:      group.GroupID,
	})
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId(hostname.EdgeHostnameID)
	return nil
}

func resourceSecureEdgeHostNameDelete(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	meta := akamai.Meta(m)
	logger := meta.Log("PAPI", "resourceSecureEdgeHostNameDelete")
	logger.Debugf("DELETING")
	logger.Info("PAPI does not support edge hostname deletion - resource will only be removed from state")
	d.SetId("")
	logger.Debugf("DONE")
	return nil
}

func resourceSecureEdgeHostNameImport(ctx context.Context, d *schema.ResourceData, m interface{}) ([]*schema.ResourceData, error) {
	meta := akamai.Meta(m)
	logger := meta.Log("PAPI", "resourceSecureEdgeHostNameImport")
	resourceID := d.Id()
	propertyID := resourceID

	client := inst.Client(meta)

	if !strings.HasPrefix(resourceID, "prp_") {
		keys := []string{
			papi.SearchKeyPropertyName,
			papi.SearchKeyHostname,
			papi.SearchKeyEdgeHostname,
		}
		for _, searchKey := range keys {
			results, err := client.SearchProperties(ctx, papi.SearchRequest{
				Key:   searchKey,
				Value: resourceID,
			})
			if err != nil {
				// TODO determine why is this error ignored
				logger.Debugf("searching by key: %s: %w", searchKey, err)
				continue
			}

			if results != nil && len(results.Versions.Items) > 0 {
				propertyID = results.Versions.Items[0].PropertyID
				break
			}
		}
	}

	prop, err := client.GetProperty(ctx, papi.GetPropertyRequest{
		PropertyID: propertyID,
	})
	if err != nil {
		return nil, err
	}

	if err := d.Set("account", prop.Property.AccountID); err != nil {
		return nil, fmt.Errorf("%w: %s", tools.ErrValueSet, err.Error())
	}
	if err := d.Set("contract", prop.Property.ContractID); err != nil {
		return nil, fmt.Errorf("%w: %s", tools.ErrValueSet, err.Error())
	}
	if err := d.Set("group", prop.Property.GroupID); err != nil {
		return nil, fmt.Errorf("%w: %s", tools.ErrValueSet, err.Error())
	}
	if err := d.Set("name", prop.Property.PropertyName); err != nil {
		return nil, fmt.Errorf("%w: %s", tools.ErrValueSet, err.Error())
	}
	if err := d.Set("version", prop.Property.LatestVersion); err != nil {
		return nil, fmt.Errorf("%w: %s", tools.ErrValueSet, err.Error())
	}
	d.SetId(prop.Property.PropertyID)

	return []*schema.ResourceData{d}, nil
}

func resourceSecureEdgeHostNameRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	meta := akamai.Meta(m)
	logger := meta.Log("PAPI", "resourceSecureEdgeHostNameCreate")

	client := inst.Client(meta)

	group, err := getGroup(ctx, d, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	contract, err := getContract(ctx, d, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	logger.Debugf("  Edgehostnames GROUP = %v", group)
	logger.Debugf("Edgehostnames CONTRACT = %v", contract)

	edgeHostnames, err := client.GetEdgeHostnames(ctx, papi.GetEdgeHostnamesRequest{
		ContractID: contract.ContractID,
		GroupID:    group.GroupID,
	})
	if err != nil {
		return diag.FromErr(err)
	}
	defaultEdgeHostname := &edgeHostnames.EdgeHostnames.Items[0]

	edgeHostname, err := tools.GetStringValue("edge_hostname", d)
	if err != nil {
		return diag.FromErr(err)
	}

	if edgeHostname != "" {
		defaultEdgeHostname, err = findEdgeHostname(edgeHostnames.EdgeHostnames, "", edgeHostname, "", "")
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if err := d.Set("contract", contract.ContractID); err != nil {
		return diag.FromErr(fmt.Errorf("%w: %s", tools.ErrValueSet, err.Error()))
	}
	if err := d.Set("group", group.GroupID); err != nil {
		return diag.FromErr(fmt.Errorf("%w: %s", tools.ErrValueSet, err.Error()))
	}
	if err := d.Set("edge_hostname", defaultEdgeHostname.Domain); err != nil {
		return diag.FromErr(fmt.Errorf("%w: %s", tools.ErrValueSet, err.Error()))
	}
	d.SetId(defaultEdgeHostname.ID)

	return nil
}
