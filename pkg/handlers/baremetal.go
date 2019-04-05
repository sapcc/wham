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
	"errors"
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
		*gophercloud.ServiceClient
		ctx context.Context
		log *log.Entry
	}

	maintenanceReason struct {
		Reason string `json:"reason"`
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

func NewBaremetalHandler(ctx context.Context) (Handler, error) {
	opts, err := openstack.AuthOptionsFromEnv()
	if err != nil {
		return nil, err
	}
	opts.AllowReauth = true
	opts.Scope = &gophercloud.AuthScope{
		ProjectName: opts.TenantName,
		DomainName:  os.Getenv("OS_PROJECT_DOMAIN_NAME"),
	}
	provider, err := openstack.AuthenticatedClient(opts)
	if err != nil {
		return nil, err
	}
	serviceType := "baremetal"
	eo := gophercloud.EndpointOpts{Availability: gophercloud.AvailabilityPublic}
	eo.ApplyDefaults(serviceType)

	url, err := provider.EndpointLocator(eo)
	log.Debug(url)
	if err != nil {
		return nil, err
	}

	if err := prometheus.Register(alertsCounter); err != nil {
		return nil, err
	}

	contextLogger := log.WithFields(log.Fields{
		"component": "baremetal_handler",
	})

	return Baremetal{
		ServiceClient: &gophercloud.ServiceClient{
			ProviderClient: provider,
			Endpoint:       url,
			Type:           serviceType,
		},
		ctx: ctx,
		log: contextLogger,
	}, nil
}

//Run imlements the handler interface
func (c Baremetal) Run(a *api.API, wg *sync.WaitGroup) error {
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

func (c Baremetal) alert(alert template.Alert) (err error) {
	nodeID, err := c.getNodeID(alert)
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

func (c Baremetal) getNodeID(a template.Alert) (nodeID string, err error) {
	r, _ := regexp.Compile("server_id: (([a-z0-9]*-){4}[a-z0-9]*)")
	meta, isset := a.Labels["meta"]
	if !isset {
		return nodeID, errors.New("Missing server id")
	}
	match := r.FindStringSubmatch(meta)
	if len(match) < 1 {
		return nodeID, errors.New("Missing server id")
	}
	nodeID = match[1]
	c.log.Debugf("found server id %s in alert", nodeID)

	return nodeID, err

}

func (c Baremetal) getNode(id string) (node *nodes.Node, err error) {
	node, err = nodes.Get(c.ServiceClient, id).Extract()
	if err != nil {
		return node, err
	}

	return node, err
}

func (c Baremetal) setNodeInMaintenance(node *nodes.Node) (err error) {
	if node.ProvisionState == nodes.Active {
		return fmt.Errorf("node %s: Cannot set Active node into maintenance", node.UUID)
	}
	updated, err := nodes.Update(c.ServiceClient, node.UUID, nodes.UpdateOpts{
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

func (c Baremetal) setNodeMaintenanceReason(id string, reason maintenanceReason) (err error) {
	url := c.ServiceClient.ServiceURL("nodes", id) + "/maintenance"
	resp, err := c.ServiceClient.Request("PUT", url, &gophercloud.RequestOpts{
		JSONBody: reason,
		OkCodes:  []int{200, 202},
	})

	defer resp.Body.Close()

	if err != nil {
		return
	}

	return
}
