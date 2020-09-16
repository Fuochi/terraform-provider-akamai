package gtm

import (
	"fmt"
	"log"

	gtm "github.com/akamai/AkamaiOPEN-edgegrid-golang/configgtm-v1_4"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceGTMv1ASmap() *schema.Resource {
	return &schema.Resource{
		Create: resourceGTMv1ASmapCreate,
		Read:   resourceGTMv1ASmapRead,
		Update: resourceGTMv1ASmapUpdate,
		Delete: resourceGTMv1ASmapDelete,
		Exists: resourceGTMv1ASmapExists,
		Importer: &schema.ResourceImporter{
			State: resourceGTMv1ASmapImport,
		},
		Schema: map[string]*schema.Schema{
			"domain": {
				Type:     schema.TypeString,
				Required: true,
			},
			"wait_on_complete": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"default_datacenter": {
				Type:       schema.TypeList,
				Required:   true,
				MaxItems:   1,
				ConfigMode: schema.SchemaConfigModeAttr,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"datacenter_id": {
							Type:     schema.TypeInt,
							Required: true,
						},
						"nickname": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			"assignment": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"datacenter_id": {
							Type:     schema.TypeInt,
							Required: true,
						},
						"nickname": {
							Type:     schema.TypeString,
							Required: true,
						},
						"as_numbers": {
							Type:     schema.TypeList,
							Elem:     &schema.Schema{Type: schema.TypeInt},
							Required: true,
						},
					},
				},
			},
		},
	}
}

// utility func to parse Terraform property asMap id
func parseResourceASmapId(id string) (string, string, error) {

	return parseResourceStringId(id)

}

// Util method to validate default datacenter and create if necessary
func validateDefaultDC(ddcField []interface{}, domain string) error {

	if len(ddcField) == 0 {
		return fmt.Errorf("default Datacenter invalid")
	}
	ddc := ddcField[0].(map[string]interface{})
	if ddc["datacenter_id"].(int) == 0 {
		return fmt.Errorf("default Datacenter ID invalid")
	}
	dc, err := gtm.GetDatacenter(ddc["datacenter_id"].(int), domain)
	if dc == nil {
		if err != nil {
			_, ok := err.(gtm.CommonError)
			if !ok {
				return fmt.Errorf("[ERROR] MapCreate Unexpected error verifying Default Datacenter exists: %s", err.Error())
			}
		}
		// ddc doesn't exist
		if ddc["datacenter_id"].(int) != gtm.MapDefaultDC {
			return fmt.Errorf(fmt.Sprintf("Default Datacenter %d does not exist", ddc["datacenter_id"].(int)))
		}
		_, err := gtm.CreateMapsDefaultDatacenter(domain) // create if not already.
		if err != nil {
			return fmt.Errorf("[ERROR] MapCreate failed on Default Datacenter check: %s", err.Error())
		}
	}

	return nil

}

// Create a new GTM ASmap
func resourceGTMv1ASmapCreate(d *schema.ResourceData, m interface{}) error {

	domain := d.Get("domain").(string)

	log.Printf("[INFO] [Akamai GTM] Creating asMap [%s] in domain [%s]", d.Get("name").(string), domain)
	// Make sure Default Datacenter exists
	err := validateDefaultDC(d.Get("default_datacenter").([]interface{}), domain)
	if err != nil {
		return err
	}

	newAS := populateNewASmapObject(d)
	log.Printf("[DEBUG] [Akamai GTMv1] Proposed New ASmap: [%v]", newAS)
	cStatus, err := newAS.Create(domain)
	if err != nil {
		log.Printf("[ERROR] [Akamai GTMv1] ASmap Create failed: %s", err.Error())
		return err
	}
	log.Printf("[DEBUG] [Akamai GTMv1] ASmap Create status:")
	log.Printf("[DEBUG] [Akamai GTMv1] %v", cStatus.Status)
	if cStatus.Status.PropagationStatus == "DENIED" {
		return fmt.Errorf(cStatus.Status.Message)
	}
	if d.Get("wait_on_complete").(bool) {
		done, err := waitForCompletion(domain)
		if done {
			log.Printf("[INFO] [Akamai GTMv1] ASmap Create completed")
		} else {
			if err == nil {
				log.Printf("[INFO] [Akamai GTMv1] ASmap Create pending")
			} else {
				log.Printf("[WARNING] [Akamai GTMv1] ASmap Create failed [%s]", err.Error())
				return err
			}
		}

	}

	// Give terraform the ID. Format domain:asMap
	asMapId := fmt.Sprintf("%s:%s", domain, cStatus.Resource.Name)
	log.Printf("[DEBUG] [Akamai GTMv1] Generated ASmap ASmap Id: %s", asMapId)
	d.SetId(asMapId)
	return resourceGTMv1ASmapRead(d, m)

}

// read asMap. updates state with entire API result configuration.
func resourceGTMv1ASmapRead(d *schema.ResourceData, m interface{}) error {

	log.Printf("[DEBUG] [Akamai GTMv1] READ")
	log.Printf("[DEBUG] Reading [Akamai GTMv1] ASmap: %s", d.Id())
	// retrieve the property and domain
	domain, asMap, err := parseResourceASmapId(d.Id())
	if err != nil {
		return fmt.Errorf("invalid asMap asMap Id")
	}
	as, err := gtm.GetAsMap(asMap, domain)
	if err != nil {
		log.Printf("[ERROR] [Akamai GTMv1] ASmap Read error: %s", err.Error())
		return err
	}
	populateTerraformASmapState(d, as)
	log.Printf("[DEBUG] [Akamai GTMv1] READ %v", as)
	return nil
}

// Update GTM ASmap
func resourceGTMv1ASmapUpdate(d *schema.ResourceData, m interface{}) error {

	log.Printf("[DEBUG] [Akamai GTMv1] UPDATE")
	log.Printf("[DEBUG] Updating [Akamai GTMv1] ASmap: %s", d.Id())
	// pull domain and asMap out of id
	domain, asMap, err := parseResourceASmapId(d.Id())
	if err != nil {
		return fmt.Errorf("invalid asMap Id")
	}
	// Get existingASmap
	existAs, err := gtm.GetAsMap(asMap, domain)
	if err != nil {
		log.Printf("[ERROR] ASmapUpdate: %s", err.Error())
		return err
	}
	log.Printf("[DEBUG] Updating [Akamai GTMv1] ASmap BEFORE: %v", existAs)
	populateASmapObject(d, existAs)
	log.Printf("[DEBUG] Updating [Akamai GTMv1] ASmap PROPOSED: %v", existAs)
	uStat, err := existAs.Update(domain)
	if err != nil {
		log.Printf("[ERROR] ASmapUpdate: %s", err.Error())
		return err
	}
	log.Printf("[DEBUG] [Akamai GTMv1] ASmap Update  status:")
	log.Printf("[DEBUG] [Akamai GTMv1] %v", uStat)
	if uStat.PropagationStatus == "DENIED" {
		return fmt.Errorf(uStat.Message)
	}
	if d.Get("wait_on_complete").(bool) {
		done, err := waitForCompletion(domain)
		if done {
			log.Printf("[INFO] [Akamai GTMv1] ASmap update completed")
		} else {
			if err == nil {
				log.Printf("[INFO] [Akamai GTMv1] ASmap update pending")
			} else {
				log.Printf("[WARNING] [Akamai GTMv1] ASmap update failed [%s]", err.Error())
				return err
			}
		}
	}

	return resourceGTMv1ASmapRead(d, m)
}

// Import GTM ASmap.
func resourceGTMv1ASmapImport(d *schema.ResourceData, m interface{}) ([]*schema.ResourceData, error) {

	log.Printf("[INFO] [Akamai GTM] ASmap [%s] Import", d.Id())
	// pull domain and asMap out of asMap id
	domain, asMap, err := parseResourceASmapId(d.Id())
	if err != nil {
		return []*schema.ResourceData{d}, fmt.Errorf("invalid asMap Id")
	}
	as, err := gtm.GetAsMap(asMap, domain)
	if err != nil {
		return nil, err
	}
	err = d.Set("domain", domain)
	if err != nil {
		return nil, err
	}
	err = d.Set("wait_on_complete", true)
	if err != nil {
		return nil, err
	}
	populateTerraformASmapState(d, as)

	// use same Id as passed in
	log.Printf("[INFO] [Akamai GTM] ASmap [%s] [%s] Imported", d.Id(), d.Get("name"))
	return []*schema.ResourceData{d}, nil
}

// Delete GTM ASmap.
func resourceGTMv1ASmapDelete(d *schema.ResourceData, m interface{}) error {

	log.Printf("[DEBUG] [Akamai GTMv1] DELETE")
	log.Printf("[DEBUG] Deleting [Akamai GTMv1] ASmap: %s", d.Id())
	// Get existing asMap
	domain, asMap, err := parseResourceASmapId(d.Id())
	if err != nil {
		log.Printf("[ERROR] ASmapDelete: %s", err.Error())
		return fmt.Errorf("invalid asMap Id")
	}
	existAs, err := gtm.GetAsMap(asMap, domain)
	if err != nil {
		log.Printf("[ERROR] ASmapDelete: %s", err.Error())
		return err
	}
	log.Printf("[DEBUG] Deleting [Akamai GTMv1] ASmap: %v", existAs)
	uStat, err := existAs.Delete(domain)
	if err != nil {
		log.Printf("[ERROR] ASmapDelete: %s", err.Error())
		return err
	}
	log.Printf("[DEBUG] [Akamai GTMv1] ASmap Delete status:")
	log.Printf("[DEBUG] [Akamai GTMv1] %v", uStat)
	if uStat.PropagationStatus == "DENIED" {
		return fmt.Errorf(uStat.Message)
	}
	if d.Get("wait_on_complete").(bool) {
		done, err := waitForCompletion(domain)
		if done {
			log.Printf("[INFO] [Akamai GTMv1] ASmap delete completed")
		} else {
			if err == nil {
				log.Printf("[INFO] [Akamai GTMv1] ASmap delete pending")
			} else {
				log.Printf("[WARNING] [Akamai GTMv1] ASmap delete failed [%s]", err.Error())
				return err
			}
		}

	}

	// if successful ....
	d.SetId("")
	return nil
}

// Test GTM ASmap existence
func resourceGTMv1ASmapExists(d *schema.ResourceData, m interface{}) (bool, error) {

	log.Printf("[DEBUG] [Akamai GTMv1] Exists")
	// pull domain and asMap out of asMap id
	domain, asMap, err := parseResourceASmapId(d.Id())
	if err != nil {
		return false, fmt.Errorf("invalid asMap Id")
	}
	log.Printf("[DEBUG] [Akamai GTMv1] Searching for existing asMap [%s] in domain %s", asMap, domain)
	as, err := gtm.GetAsMap(asMap, domain)
	return as != nil, err
}

// Create and populate a new asMap object from asMap data
func populateNewASmapObject(d *schema.ResourceData) *gtm.AsMap {

	asObj := gtm.NewAsMap(d.Get("name").(string))
	asObj.DefaultDatacenter = &gtm.DatacenterBase{}
	asObj.Assignments = make([]*gtm.AsAssignment, 1)
	asObj.Links = make([]*gtm.Link, 1)
	populateASmapObject(d, asObj)

	return asObj

}

// Populate existing asMap object from asMap data
func populateASmapObject(d *schema.ResourceData, as *gtm.AsMap) {

	if v, ok := d.GetOk("name"); ok {
		as.Name = v.(string)
	}
	populateAsAssignmentsObject(d, as)
	populateAsDefaultDCObject(d, as)

}

// Populate Terraform state from provided ASmap object
func populateTerraformASmapState(d *schema.ResourceData, as *gtm.AsMap) {

	// walk through all state elements
	err := d.Set("name", as.Name)
	if err != nil {
		log.Printf("[ERROR] populateTerraformASmapState failed: %s", err.Error())
	}
	populateTerraformAsAssignmentsState(d, as)
	populateTerraformAsDefaultDCState(d, as)
}

// create and populate GTM ASmap Assignments object
func populateAsAssignmentsObject(d *schema.ResourceData, as *gtm.AsMap) {

	// pull apart List
	assgn := d.Get("assignment")
	if assgn != nil {
		asAssignmentsList := assgn.([]interface{})
		asAssignmentsObjList := make([]*gtm.AsAssignment, len(asAssignmentsList)) // create new object list
		for i, v := range asAssignmentsList {
			asMap := v.(map[string]interface{})
			asAssignment := gtm.AsAssignment{}
			asAssignment.DatacenterId = asMap["datacenter_id"].(int)
			asAssignment.Nickname = asMap["nickname"].(string)
			if asMap["as_numbers"] != nil {
				ls := make([]int64, len(asMap["as_numbers"].([]interface{})))
				for i, sl := range asMap["as_numbers"].([]interface{}) {
					ls[i] = int64(sl.(int))
				}
				asAssignment.AsNumbers = ls
			}
			asAssignmentsObjList[i] = &asAssignment
		}
		as.Assignments = asAssignmentsObjList
	}
}

// create and populate Terraform asMap assigments schema
func populateTerraformAsAssignmentsState(d *schema.ResourceData, as *gtm.AsMap) {

	objectInventory := make(map[int]*gtm.AsAssignment, len(as.Assignments))
	if len(as.Assignments) > 0 {
		for _, aObj := range as.Assignments {
			objectInventory[aObj.DatacenterId] = aObj
		}
	}
	aStateList := d.Get("assignment").([]interface{})
	for _, aMap := range aStateList {
		a := aMap.(map[string]interface{})
		objIndex := a["datacenter_id"].(int)
		aObject := objectInventory[objIndex]
		if aObject == nil {
			log.Printf("[WARNING] [Akamai GTMv1] As Assignment %d NOT FOUND in returned GTM Object", a["datacenter_id"])
			continue
		}
		a["datacenter_id"] = aObject.DatacenterId
		a["nickname"] = aObject.Nickname
		a["as_numbers"] = reconcileTerraformLists(a["as_numbers"].([]interface{}), convertInt64ToInterfaceList(aObject.AsNumbers))
		// remove object
		delete(objectInventory, objIndex)
	}
	if len(objectInventory) > 0 {
		log.Printf("[DEBUG] [Akamai GTMv1] As Assignment objects left...")
		// Objects not in the state yet. Add. Unfortunately, they not align with instance indices in the config
		for _, maObj := range objectInventory {
			aNew := map[string]interface{}{
				"datacenter_id": maObj.DatacenterId,
				"nickname":      maObj.Nickname,
				"as_numbers":    maObj.AsNumbers,
			}
			aStateList = append(aStateList, aNew)
		}
	}
	err := d.Set("assignment", aStateList)
	if err != nil {
		log.Printf("[ERROR] populateTerraformAsAssignmentsState failed: %s", err.Error())
	}
}

// create and populate GTM ASmap DefaultDatacenter object
func populateAsDefaultDCObject(d *schema.ResourceData, as *gtm.AsMap) {

	// pull apart List
	asm := d.Get("default_datacenter")
	if asm != nil && len(asm.([]interface{})) > 0 {
		asDefaultDCObj := gtm.DatacenterBase{} // create new object
		asDefaultDCList := asm.([]interface{})
		asMap := asDefaultDCList[0].(map[string]interface{})
		if asMap["datacenter_id"] != nil && asMap["datacenter_id"].(int) != 0 {
			asDefaultDCObj.DatacenterId = asMap["datacenter_id"].(int)
			asDefaultDCObj.Nickname = asMap["nickname"].(string)
		} else {
			log.Printf("[INFO] [Akamai GTMv1] No Default Datacenter specified")
			var nilInt int
			asDefaultDCObj.DatacenterId = nilInt
			asDefaultDCObj.Nickname = ""
		}
		as.DefaultDatacenter = &asDefaultDCObj
	}
}

// create and populate Terraform asMap default_datacenter schema
func populateTerraformAsDefaultDCState(d *schema.ResourceData, as *gtm.AsMap) {

	ddcListNew := make([]interface{}, 1)
	ddcNew := map[string]interface{}{
		"datacenter_id": as.DefaultDatacenter.DatacenterId,
		"nickname":      as.DefaultDatacenter.Nickname,
	}
	ddcListNew[0] = ddcNew
	err := d.Set("default_datacenter", ddcListNew)
	if err != nil {
		log.Printf("[ERROR] populateTerraformAsDefaultDCState failed: %s", err.Error())
	}
}