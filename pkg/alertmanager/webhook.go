package alertmanager

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

type (
	responseJSON struct {
		Status  int
		Message string
	}
)

var ctxLog = log.WithFields(log.Fields{
	"component": "webhook",
})

func HandleWebhookAlerts(counter prometheus.Counter, alerts chan<- template.Alert) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if r.Body == nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		data := template.Data{}
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			toJSON(w, http.StatusBadRequest, err.Error())
			return
		}

		for _, alert := range data.Alerts {
			ctxLog.Debugf("Alert: status=%s,Labels=%v,Annotations=%v", alert.Status, alert.Labels, alert.Annotations)
			alerts <- alert
		}
		toJSON(w, http.StatusOK, "success")

		counter.Inc()
	}
}

func toJSON(w http.ResponseWriter, status int, message string) {
	data := responseJSON{
		Status:  status,
		Message: message,
	}
	bytes, _ := json.Marshal(data)
	json := string(bytes[:])

	w.WriteHeader(status)
	fmt.Fprint(w, json)
}
