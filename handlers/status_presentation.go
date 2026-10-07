package handlers

import "barber-booking/models"

type statusPresentation struct{ Label, Tone, Description string }

// Stored values remain domain-owned; this catalog only controls visible labels
// and treatments. Unknown values never become arbitrary CSS class names.
var statusCatalog = map[string]map[string]statusPresentation{
	"booking": {
		models.StatusPending:   {"Pending", "warning", "Waiting for the barber to accept or reject this appointment request."},
		models.StatusAccepted:  {"Accepted", "positive", "The barber has accepted this appointment."},
		models.StatusRejected:  {"Rejected", "danger", "The barber has declined this appointment request."},
		models.StatusCancelled: {"Cancelled", "muted", "This appointment has been cancelled."},
		models.StatusCompleted: {"Completed", "positive", "This appointment has been marked completed."},
	},
	"account": {
		models.BarberActive:    {"Active", "positive", "This barber account is active."},
		models.BarberSuspended: {"Suspended", "warning", "This barber account is suspended and hidden from discovery."},
		models.BarberArchived:  {"Archived", "muted", "This barber account is archived; its history is retained."},
	},
	"order": {
		models.OrderPending:   {"Pending", "warning", "This order contains items awaiting preparation."},
		models.OrderPaid:      {"Paid", "info", "This order has been recorded as paid."},
		models.OrderReady:     {"Ready", "info", "This order contains items ready for collection. Check each item's status."},
		models.OrderCancelled: {"Cancelled", "muted", "All items in this order have been cancelled."},
		models.OrderCompleted: {"Completed", "positive", "Collection is complete for all non-cancelled items in this order."},
	},
	"fulfillment": {
		models.FulfillmentPending:   {"Pending", "warning", "This item is awaiting preparation."},
		models.FulfillmentReady:     {"Ready", "info", "This item is ready for collection."},
		models.FulfillmentCollected: {"Collected", "positive", "This item has been collected."},
		models.FulfillmentCancelled: {"Cancelled", "muted", "This item has been cancelled."},
	},
}

func domainStatus(domain, value string) statusPresentation {
	if status, ok := statusCatalog[domain][value]; ok {
		return status
	}
	return statusPresentation{"Unknown status", "muted", "This status is not recognized. Refresh the page or contact the shop."}
}
func availabilityStatus(available bool) statusPresentation {
	if available {
		return statusPresentation{"Available", "positive", "This listing is available to customers."}
	}
	return statusPresentation{"Hidden from customers", "muted", "This listing is hidden from customers."}
}
