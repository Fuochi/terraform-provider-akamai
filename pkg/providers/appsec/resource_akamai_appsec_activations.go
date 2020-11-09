package appsec

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	v2 "github.com/akamai/AkamaiOPEN-edgegrid-golang/v2/pkg/appsec"
	"github.com/akamai/terraform-provider-akamai/v2/pkg/akamai"
	"github.com/akamai/terraform-provider-akamai/v2/pkg/tools"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// appsec v1
//
// https://developer.akamai.com/api/cloud_security/application_security/v1.html
func resourceActivations() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceActivationsCreate,
		ReadContext:   resourceActivationsRead,
		DeleteContext: resourceActivationsDelete,
		Schema: map[string]*schema.Schema{
			"config_id": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"version": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"network": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Default:  "STAGING",
			},
			"notes": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Default:  "Activation Notes",
			},
			"activate": {
				Type:     schema.TypeBool,
				Optional: true,
				ForceNew: true,
				Default:  true,
			},
			"notification_emails": {
				Type:     schema.TypeSet,
				Required: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"status": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

const (
	// ActivationPollMinimum is the minumum polling interval for activation creation
	ActivationPollMinimum = time.Minute
)

var (
	// ActivationPollInterval is the interval for polling an activation status on creation
	ActivationPollInterval = ActivationPollMinimum
)

func resourceActivationsCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	meta := akamai.Meta(m)
	client := inst.Client(meta)
	logger := meta.Log("APPSEC", "resourceActivationsCreate")

	activate, err := tools.GetBoolValue("activate", d)
	if err != nil {
		return diag.FromErr(err)
	}
	if !activate {
		d.SetId("none")
		logger.Debugf("Done")
		return nil
	}

	createActivations := v2.CreateActivationsRequest{}

	createActivationsreq := v2.GetActivationsRequest{}

	ap := v2.ActivationConfigs{}

	configid, err := tools.GetIntValue("config_id", d)
	if err != nil && !errors.Is(err, tools.ErrNotFound) {
		return diag.FromErr(err)
	}
	ap.ConfigID = configid

	version, err := tools.GetIntValue("version", d)
	if err != nil && !errors.Is(err, tools.ErrNotFound) {
		return diag.FromErr(err)
	}
	ap.ConfigVersion = version

	network, err := tools.GetStringValue("network", d)
	if err != nil && !errors.Is(err, tools.ErrNotFound) {
		return diag.FromErr(err)
	}
	createActivations.Network = network

	createActivations.Action = "ACTIVATE"
	createActivations.ActivationConfigs = append(createActivations.ActivationConfigs, ap)
	createActivations.NotificationEmails = tools.SetToStringSlice(d.Get("notification_emails").(*schema.Set))

	postresp, err := client.CreateActivations(ctx, createActivations, true)
	if err != nil {
		logger.Errorf("calling 'createActivations': %s", err.Error())
		return diag.FromErr(err)
	}

	d.SetId(strconv.Itoa(postresp.ActivationID))
	d.Set("status", string(postresp.Status))

	createActivationsreq.ActivationID = postresp.ActivationID
	activation, err := lookupActivation(ctx, client, createActivationsreq)
	for activation.Status != v2.StatusActive {
		select {
		case <-time.After(tools.MaxDuration(ActivationPollInterval, ActivationPollMinimum)):
			act, err := client.GetActivations(ctx, createActivationsreq)

			if err != nil {
				return diag.FromErr(err)
			}
			activation = act

		case <-ctx.Done():
			return diag.FromErr(fmt.Errorf("activation context terminated: %w", ctx.Err()))
		}
	}

	return resourceActivationsRead(ctx, d, m)
}

func resourceActivationsDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	meta := akamai.Meta(m)
	client := inst.Client(meta)
	logger := meta.Log("APPSEC", "resourceActivationsRemove")

	activate, err := tools.GetBoolValue("activate", d)
	if err != nil {
		return diag.FromErr(err)
	}
	if !activate {
		d.SetId("none")
		logger.Debugf("Done")
		return nil
	}

	removeActivations := v2.RemoveActivationsRequest{}
	createActivationsreq := v2.GetActivationsRequest{}

	if d.Id() == "" {
		return nil
	}

	removeActivations.ActivationID, _ = strconv.Atoi(d.Id())
	//postpayload := appsec.NewActivationsPost()
	ap := v2.ActivationConfigs{}

	configid, err := tools.GetIntValue("config_id", d)
	if err != nil && !errors.Is(err, tools.ErrNotFound) {
		return diag.FromErr(err)
	}
	ap.ConfigID = configid

	version, err := tools.GetIntValue("version", d)
	if err != nil && !errors.Is(err, tools.ErrNotFound) {
		return diag.FromErr(err)
	}
	ap.ConfigVersion = version

	network, err := tools.GetStringValue("network", d)
	if err != nil && !errors.Is(err, tools.ErrNotFound) {
		return diag.FromErr(err)
	}
	removeActivations.Network = network

	removeActivations.NotificationEmails = tools.SetToStringSlice(d.Get("notification_emails").(*schema.Set))

	removeActivations.Action = "DEACTIVATE"

	removeActivations.ActivationConfigs = append(removeActivations.ActivationConfigs, ap)

	postresp, err := client.RemoveActivations(ctx, removeActivations)

	if err != nil {
		logger.Errorf("calling 'removeActivations': %s", err.Error())
		return diag.FromErr(err)
	}

	d.SetId(strconv.Itoa(postresp.ActivationID))
	d.Set("status", string(postresp.Status))
	createActivationsreq.ActivationID = postresp.ActivationID

	activation, err := lookupActivation(ctx, client, createActivationsreq)
	for activation.Status != v2.StatusDeactivated {
		select {
		case <-time.After(tools.MaxDuration(ActivationPollInterval, ActivationPollMinimum)):
			act, err := client.GetActivations(ctx, createActivationsreq)

			if err != nil {
				return diag.FromErr(err)
			}
			activation = act

		case <-ctx.Done():
			return diag.FromErr(fmt.Errorf("activation context terminated: %w", ctx.Err()))
		}
	}

	d.Set("status", string(activation.Status))

	d.SetId("")

	return nil
}

func resourceActivationsRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	meta := akamai.Meta(m)
	client := inst.Client(meta)
	logger := meta.Log("APPSEC", "resourceActivationsRead")

	getActivations := v2.GetActivationsRequest{}

	getActivations.ActivationID, _ = strconv.Atoi(d.Id())

	activations, err := client.GetActivations(ctx, getActivations)
	if err != nil {
		logger.Errorf("calling 'getActivations': %s", err.Error())
		return diag.FromErr(err)
	}

	d.Set("status", activations.Status)
	d.SetId(strconv.Itoa(activations.ActivationID))

	return nil
}

func lookupActivation(ctx context.Context, client v2.APPSEC, query v2.GetActivationsRequest) (*v2.GetActivationsResponse, error) {
	activations, err := client.GetActivations(ctx, query)
	if err != nil {
		return nil, err
	}

	// There is an activation in progress, if it's for the same version/network/type we can re-use it
	//if activations.Action == query.activationType && activations.Network == query.network {
	return activations, nil
	//}

	return nil, nil
}
