// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package federation

import (
	"fmt"
	"io"
	"net/url"

	"forgejo.org/models/user"
	"forgejo.org/modules/activitypub"
	"forgejo.org/modules/graceful"
	"forgejo.org/modules/log"
	"forgejo.org/modules/process"
	"forgejo.org/modules/queue"
)

type deliveryQueueItem struct {
	Doer          *user.User
	InboxURL      string
	Payload       []byte
	DeliveryCount int
}

var deliveryQueue *queue.WorkerPoolQueue[deliveryQueueItem]

func initDeliveryQueue() error {
	deliveryQueue = queue.CreateUniqueQueue(graceful.GetManager().ShutdownContext(), "activitypub_inbox_delivery", deliveryQueueHandler)
	if deliveryQueue == nil {
		return fmt.Errorf("unable to create activitypub_inbox_delivery queue")
	}
	go graceful.GetManager().RunWithCancel(deliveryQueue)

	return nil
}

func deliveryQueueHandler(items ...deliveryQueueItem) (unhandled []deliveryQueueItem) {
	for _, item := range items {
		item.DeliveryCount++
		err := deliverToInbox(item)
		if err != nil && item.DeliveryCount < 10 {
			unhandled = append(unhandled, item)
		}
	}
	return unhandled
}

func deliverToInbox(item deliveryQueueItem) error {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(),
		fmt.Sprintf("Delivering an Activity via user[%d] (%s), to %s", item.Doer.ID, item.Doer.Name, item.InboxURL))
	defer finished()

	clientFactory, err := activitypub.GetClientFactory(ctx)
	if err != nil {
		return err
	}

	inboxURL, err := url.Parse(item.InboxURL)
	if err != nil {
		return fmt.Errorf("invalid delivery item inbox URL: %w", err)
	}

	apclient, err := clientFactory.WithKeys(ctx, item.Doer, item.Doer.APActorID()+"#main-key", []*url.URL{inboxURL})
	if err != nil {
		return err
	}

	log.Trace("Delivering to: %s, signedBy: %s", item.InboxURL, item.Doer.ID)
	res, err := apclient.Post(item.Payload, item.InboxURL)
	if err != nil {
		log.Info("Delivering to: %s failed: %s, times: %v", item.InboxURL, err, item.DeliveryCount)
		return err
	}
	if res.StatusCode >= 400 {
		defer res.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(res.Body, 16*1024))

		log.Warn("Delivering to: %s failed. Status: %d, responseBody: %s, times: %v", item.InboxURL, res.StatusCode, string(body), item.DeliveryCount)
		return fmt.Errorf("delivery failed")
	}

	return nil
}
