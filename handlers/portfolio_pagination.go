package handlers

import (
	"barber-booking/models"
	"barber-booking/storage"
	"net/http"
	"strings"
)

type portfolioPages struct {
	Services                                storage.Page[models.Hairstyle]
	Products                                storage.Page[models.Product]
	Reviews                                 storage.Page[models.Review]
	ServiceLinks, ProductLinks, ReviewLinks listPagination
	Rating                                  float64
	ReviewCount                             int
}

func inventoryPagination(r *http.Request, c storage.PageCursor, items []models.Hairstyle, previous, next bool) listPagination {
	first, last := 0, 0
	if len(items) > 0 {
		first, last = items[0].ID, items[len(items)-1].ID
	}
	return paginationLinks(r, c, first, last, true, previous, next)
}
func productInventoryPagination(r *http.Request, c storage.PageCursor, items []models.Product, previous, next bool) listPagination {
	first, last := 0, 0
	if len(items) > 0 {
		first, last = items[0].ID, items[len(items)-1].ID
	}
	return paginationLinks(r, c, first, last, true, previous, next)
}
func readPortfolio(w http.ResponseWriter, r *http.Request, barberID int) (portfolioPages, bool) {
	var result portfolioPages
	service, ok := readNamedPageCursor(w, r, "services")
	if !ok {
		return result, false
	}
	product, ok := readNamedPageCursor(w, r, "products")
	if !ok {
		return result, false
	}
	review, ok := readNamedPageCursor(w, r, "reviews")
	if !ok {
		return result, false
	}
	var err error
	result.Services, err = storage.HairstylePage(r.Context(), barberID, true, service)
	if err != nil {
		serverError(w, "load portfolio services", err)
		return result, false
	}
	result.Products, err = storage.ProductPage(r.Context(), barberID, true, product)
	if err != nil {
		serverError(w, "load portfolio products", err)
		return result, false
	}
	result.Reviews, err = storage.ReviewPage(r.Context(), barberID, review)
	if err != nil {
		serverError(w, "load portfolio reviews", err)
		return result, false
	}
	result.Rating, result.ReviewCount, err = storage.ReviewSummaryResult(r.Context(), barberID)
	if err != nil {
		serverError(w, "load portfolio rating", err)
		return result, false
	}
	first, last := 0, 0
	if len(result.Services.Items) > 0 {
		first, last = result.Services.Items[0].ID, result.Services.Items[len(result.Services.Items)-1].ID
	}
	serviceFragment := "booking"
	if strings.HasPrefix(r.URL.Path, "/explore/") {
		serviceFragment = "handwork"
	}
	result.ServiceLinks = namedPaginationLinks(r, "services", serviceFragment, service, first, last, true, result.Services.HasPrevious, result.Services.HasNext)
	first, last = 0, 0
	if len(result.Products.Items) > 0 {
		first, last = result.Products.Items[0].ID, result.Products.Items[len(result.Products.Items)-1].ID
	}
	result.ProductLinks = namedPaginationLinks(r, "products", "products", product, first, last, true, result.Products.HasPrevious, result.Products.HasNext)
	first, last = 0, 0
	if len(result.Reviews.Items) > 0 {
		first, last = result.Reviews.Items[0].ID, result.Reviews.Items[len(result.Reviews.Items)-1].ID
	}
	result.ReviewLinks = namedPaginationLinks(r, "reviews", "reviews", review, first, last, false, result.Reviews.HasPrevious, result.Reviews.HasNext)
	for _, p := range []struct {
		cursor storage.PageCursor
		length int
		links  listPagination
	}{{service, len(result.Services.Items), result.ServiceLinks}, {product, len(result.Products.Items), result.ProductLinks}, {review, len(result.Reviews.Items), result.ReviewLinks}} {
		if p.length == 0 && (p.cursor.After > 0 || p.cursor.Before > 0) {
			http.Redirect(w, r, p.links.FirstLink, http.StatusSeeOther)
			return result, false
		}
	}
	return result, true
}
