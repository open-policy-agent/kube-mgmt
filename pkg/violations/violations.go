package violations

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/pkg/api/v1"
)

var source = v1.EventSource{"violation-watch", "kube-mgmt"}

type violationKey struct {
	kind string
	name string
}

type violation struct {
	Kind    string
	Name    string
	Message interface{}
}

type notification struct {
	Result []violation
}

// Track listens for violation updates through the json Decoder and posts new kubernetes
// events describing them.
func Track(decoder *json.Decoder, kube corev1.EventInterface) {
	events := map[violationKey]*v1.Event{}
	current, err := kube.List(metav1.ListOptions{})
	if err != nil {
		logrus.WithError(err).Error("Error fetching current events")
		return
	}

	for _, e := range current.Items {
		if e.Source == source {
			events[violationKey{e.TypeMeta.Kind, e.ObjectMeta.Name}] = &e
		}
	}

	for {
		var violations notification
		err := decoder.Decode(&violations)
		if err != nil {
			logrus.WithError(err).Error("Error decoding watch notification")
			continue
		}

		newEvents := map[violationKey]*v1.Event{}
		for _, v := range violations.Result {
			logrus.WithField("violation document", v).Info("New violation document value received.")

			k := violationKey{v.Kind, v.Name}
			e, ok := events[k]
			if !ok {
				initTime := metav1.Time{time.Now()}
				e = &v1.Event{
					TypeMeta:       metav1.TypeMeta{Kind: v.Kind},
					ObjectMeta:     metav1.ObjectMeta{Name: v.Name},
					Reason:         "violation notification",
					InvolvedObject: v1.ObjectReference{Kind: v.Kind, Namespace: "opa", Name: v.Name},
					FirstTimestamp: initTime,
					LastTimestamp:  initTime,
					Count:          1,
					Type:           "Normal",
					Source:         source,
					Message:        fmt.Sprint(v.Message),
				}

				if newEvents[k], err = kube.Create(e); err != nil {
					logrus.WithError(err).Error("Failed to post event")
					continue
				}
				continue
			}

			e.LastTimestamp = metav1.Time{time.Now()}
			e.Message = fmt.Sprint(v.Message)
			e.Count = e.Count + 1
			if newEvents[k], err = kube.Update(e); err != nil {
				logrus.WithError(err).Error("Failed to update event")
				continue
			}
		}

		for key, event := range events {
			if _, ok := newEvents[key]; !ok {
				kube.Delete(event.ObjectMeta.Name, &metav1.DeleteOptions{})
			}
		}
		events = newEvents
	}
}
