package handlers

import (
	"errors"
	"net/http"

	"barber-booking/models"
	"barber-booking/storage"
)

type customerProductRow = storage.CatalogueProduct

type customerProductsView struct {
	Pagination listPagination
	SellerName string
	SellerID   int
	Products   []customerProductRow
	CSRF       string
	CartCount  int
}

func CustomerProductsHandler(
	w http.ResponseWriter,
	r *http.Request,
	customer models.Customer,
	s sessionData,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	productID := 0
	if value := r.URL.Query().Get("product"); value != "" {
		var ok bool
		productID, ok = parseID(value)
		if !ok {
			journeyError(w, "Product not found.", http.StatusNotFound)
			return
		}
	}
	sellerID := 0
	sellerName := ""
	if value := r.URL.Query().Get("barber"); value != "" {
		var ok bool
		sellerID, ok = parseID(value)
		if !ok {
			journeyError(w, "Page not found.", http.StatusNotFound)
			return
		}
		seller, err := storage.FindBarberByIDContext(r.Context(), sellerID)
		if errors.Is(err, storage.ErrNotFound) || (err == nil && !seller.IsActive()) {
			journeyError(w, "Page not found.", http.StatusNotFound)
			return
		}
		if err != nil {
			serverError(w, "load catalogue seller", err)
			return
		}
		sellerName = seller.ShopName
	}
	cursor, ok := readPageCursor(w, r)
	if !ok {
		return
	}
	page, err := storage.CataloguePage(r.Context(), sellerID, productID, cursor)
	if err != nil {
		serverError(w, "load product catalogue", err)
		return
	}
	pagination := listPagination{FirstLink: paginationLinks(r, cursor, 0, 0, true, false, false).FirstLink}
	if len(page.Items) > 0 {
		pagination = paginationLinks(r, cursor, page.Items[0].Product.ID, page.Items[len(page.Items)-1].Product.ID, true, page.HasPrevious, page.HasNext)
	} else if cursor.After > 0 || cursor.Before > 0 {
		http.Redirect(w, r, pagination.FirstLink, http.StatusSeeOther)
		return
	}
	cartCount, err := storage.CartItemCount(r.Context(), customer.ID)
	if err != nil {
		serverError(w, "load cart count", err)
		return
	}

	if productID > 0 && len(page.Items) == 0 {
		journeyError(w, "This product is no longer available.", http.StatusNotFound)
		return
	}
	view := customerProductsView{
		Products:   page.Items,
		Pagination: pagination,
		SellerName: sellerName,
		SellerID:   sellerID,
		CSRF:       s.CSRFToken,
		CartCount:  cartCount,
	}

	render(w, "customer/products.html", view)
}
