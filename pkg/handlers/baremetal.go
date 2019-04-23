/*******************************************************************************
*
* Copyright 2019 SAP SE
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You should have received a copy of the License along with this
* program. If not, you may obtain a copy of the License at
*
*     http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
*
*******************************************************************************/

package handlers

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/baremetal/v1/nodes"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sapcc/wham/pkg/alertmanager"
	"github.com/sapcc/wham/pkg/api"
	log "github.com/sirupsen/logrus"
)

type (
	Baremetal struct {
		client *gophercloud.ServiceClient
		ctx    context.Context
		log    *log.Entry
		cfg    bmConfig
	}

	maintenanceReason struct {
		Reason string `json:"reason"`
	}

	bmConfig struct {
		Regions map[string]region `yaml:"regions"`
	}

	region struct {
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		AuthURL  string `yaml:"auth_url"`
	}
)

var (
	maintenanceReasonText = "IPMI Hardware Error Alert. Please check alerts in channel: alert-metal-info"
	alertsCounter         = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "monsoon3",
		Name:      "am_webhooks_bm_total",
		Help:      "Number of webhooks received by this handler",
	})
)

func init() {
	Register("baremetal", NewBaremetalHandler)

}

func NewBaremetalHandler(ctx context.Context, handler interface{}) (Handler, error) {
	var cfg bmConfig
	if err := UnmarshalHandler(handler, &cfg); err != nil {
		return nil, err
	}

	if err := prometheus.Register(alertsCounter); err != nil {
		return nil, err
	}

	contextLogger := log.WithFields(log.Fields{
		"component": "baremetal_handler",
	})

	return &Baremetal{
		ctx: ctx,
		log: contextLogger,
		cfg: cfg,
	}, nil
}

//Run imlements the handler interface
func (c *Baremetal) Run(a *api.API, wg *sync.WaitGroup) error {
	wg.Add(1)
	defer wg.Done()
	alerts := make(chan template.Alert)
	a.AddRoute("POST", "/metal", alertmanager.HandleWebhookAlerts(alertsCounter, alerts))

	for {
		select {
		case <-c.ctx.Done():
			return nil
		case a := <-alerts:
			service := a.Labels["service"]
			severity := a.Labels["severity"]
			c.log.Debugf("New Alert: Service: %s, Severity %s ", service, severity)
			switch strings.ToUpper(severity) {
			case "CRITICAL":
				if err := c.alert(a); err != nil {
					c.log.Error(err)
				}
			case "WARNING":
				if err := c.alert(a); err != nil {
					c.log.Error(err)
				}
			default:
				c.log.Debugf("no action on severity: %s", severity)
			}
		}
	}
}

func (c *Baremetal) alert(a template.Alert) (err error) {
	name := a.Labels["alertname"]
	region, isset := a.Labels["region"]
	if !isset {
		return fmt.Errorf("No region set in alert %s", name)
	}
	if err := c.setClient(region); err != nil {
		return err
	}

	nodeID, err := c.getNodeID(a)
	if err != nil {
		return
	}
	node, err := c.getNode(nodeID)
	if err != nil {
		return
	}
	if err := c.setNodeInMaintenance(node); err != nil {
		return err
	}
	return
}

func (c *Baremetal) getNodeID(a template.Alert) (nodeID string, err error) {
	name := a.Labels["alertname"]
	r, _ := regexp.Compile("server_id: (([a-z0-9]*-){4}[a-z0-9]*)")
	meta, isset := a.Labels["meta"]
	if !isset {
		return nodeID, fmt.Errorf("Missing server id in alert %s", name)
	}
	match := r.FindStringSubmatch(meta)
	if len(match) < 1 {
		return nodeID, fmt.Errorf("Missing server id in alert %s", name)
	}
	nodeID = match[1]
	c.log.Debugf("found server id %s in alert", nodeID)

	return nodeID, err

}

func (c *Baremetal) setClient(region string) (err error) {

	cfg := c.cfg.Regions[region]

	c.log.Debug(region, cfg, cfg.AuthURL, c.cfg.Regions)

	os.Setenv("OS_AUTH_URL", cfg.AuthURL)
	os.Setenv("OS_USERNAME", cfg.User)
	os.Setenv("OS_PASSWORD", cfg.Password)

	c.log.Debug(os.Getenv("OS_AUTH_URL"))

	opts, err := openstack.AuthOptionsFromEnv()
	if err != nil {
		return
	}
	opts.AllowReauth = true
	opts.Scope = &gophercloud.AuthScope{
		ProjectName: opts.TenantName,
		DomainName:  os.Getenv("OS_PROJECT_DOMAIN_NAME"),
	}
	provider, err := openstack.AuthenticatedClient(opts)
	if err != nil {
		return
	}
	serviceType := "baremetal"
	eo := gophercloud.EndpointOpts{Availability: gophercloud.AvailabilityPublic}
	eo.ApplyDefaults(serviceType)

	url, err := provider.EndpointLocator(eo)

	if err != nil {
		return
	}

	c.client = &gophercloud.ServiceClient{
		ProviderClient: provider,
		Endpoint:       url,
		Type:           serviceType,
	}

	return
}

func (c *Baremetal) getNode(id string) (node *nodes.Node, err error) {
	node, err = nodes.Get(c.client, id).Extract()
	if err != nil {
		return node, err
	}

	return node, err
}

func (c *Baremetal) setNodeInMaintenance(node *nodes.Node) (err error) {
	if node.ProvisionState == nodes.Active {
		return fmt.Errorf("node %s: Cannot set Active node into maintenance", node.UUID)
	}
	updated, err := nodes.Update(c.client, node.UUID, nodes.UpdateOpts{
		nodes.UpdateOperation{
			Op:    nodes.ReplaceOp,
			Path:  "/maintenance",
			Value: "true",
		},
	}).Extract()

	if err == nil && updated.Maintenance {
		c.log.Infof("node %s: successfuly put into maintenance", node.UUID)
	} else {
		if err == nil {
			return fmt.Errorf("node %s: unable to into maintenace", node.UUID)
		}
		return
	}

	err = c.setNodeMaintenanceReason(node.UUID, maintenanceReason{
		Reason: maintenanceReasonText,
	})

	if err == nil {
		c.log.Infof("node %s: successfuly set maintenance_reason", node.UUID)
	} else {
		return fmt.Errorf("Could not set node: %s maintenance reason. Error %s", node.UUID, err.Error())
	}

	return
}

func (c *Baremetal) setNodeMaintenanceReason(id string, reason maintenanceReason) (err error) {
	url := c.client.ServiceURL("nodes", id) + "/maintenance"
	resp, err := c.client.Request("PUT", url, &gophercloud.RequestOpts{
		JSONBody: reason,
		OkCodes:  []int{200, 202},
	})

	defer resp.Body.Close()

	if err != nil {
		return
	}

	return
}
