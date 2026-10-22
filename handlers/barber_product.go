package handlers

import (
	"barber-booking/internal/money"
	"errors"
	"net/http"
	"strconv"

	"barber-booking/models"
	"barber-booking/storage"
)

type productForm struct {
	Name        string
	Category    string
	Description string
	Price       float64
	Quantity    int
	Available   bool

	values map[string]string
}

func readProductForm(r *http.Request) (productForm, string) {
	priceText := trimmedField(r.PostFormValue("price"))
	quantityText := trimmedField(r.PostFormValue("quantity"))

	form := productForm{
		Name:        trimmedField(r.PostFormValue("name")),
		Category:    trimmedField(r.PostFormValue("category")),
		Description: truncateText(r.PostFormValue("description")),
		Available:   r.PostFormValue("available") == "on",
	}

	form.values = map[string]string{
		"name":        form.Name,
		"category":    form.Category,
		"description": form.Description,
		"price":       priceText,
		"quantity":    quantityText,
	}

	if form.Name == "" ||
		form.Category == "" ||
		form.Description == "" ||
		priceText == "" ||
		quantityText == "" {

		return form, "Every field except the photo and video is required."
	}

	price, err := money.Parse(priceText)
	if err != nil {
		return form, "Enter a price between 0 and 9999999999.99 with at most two decimal places."
	}
	form.Price = price.Float()

	quantity, err := strconv.Atoi(quantityText)

	if err != nil || quantity < 0 {
		return form, "Enter a quantity of zero or more."
	}

	form.Quantity = quantity

	return form, ""
}

type productFormView struct {
	Editing   bool
	ID        int
	Image     string
	Video     string
	Available bool

	Values map[string]string
	CSRF   string
	Error  string
}

func renderProductForm(
	w http.ResponseWriter,
	s sessionData,
	form productForm,
	id int,
	image string,
	video string,
	problem string,
) {
	status := http.StatusOK

	if problem != "" {
		status = http.StatusBadRequest
	}

	renderStatus(
		w,
		status,
		"barber/product-form.html",
		productFormView{
			Editing:   id > 0,
			ID:        id,
			Image:     image,
			Video:     video,
			Available: form.Available,
			Values:    form.values,
			CSRF:      s.CSRFToken,
			Error:     problem,
		},
	)
}

func NewProductHandler(
	w http.ResponseWriter,
	r *http.Request,
	barber models.Barber,
	s sessionData,
) {
	switch r.Method {

	case http.MethodGet:
		renderProductForm(
			w,
			s,
			productForm{
				Available: true,
			},
			0,
			"",
			"",
			"",
		)

	case http.MethodPost:
		// A large photo or clip can take minutes on a mobile connection.
		extendUploadDeadline(w)

		// A product can carry one photo and one short clip, plus the form
		// fields themselves.
		limitFormBody(
			w,
			r,
			maxUploadBytes+maxVideoBytes+maxFormMemory,
		)

		if !requireCSRF(w, r, s) {
			return
		}

		form, problem := readProductForm(r)

		if problem != "" {
			renderProductForm(w, s, form, 0, "", "", problem)
			return
		}

		image, err := productImages.save(r, "image")

		if err != nil && !errors.Is(err, errNoUpload) {
			renderProductForm(w, s, form, 0, "", "", err.Error())
			return
		}

		video, err := productVideos.save(r, "video")

		if err != nil && !errors.Is(err, errNoUpload) {
			removeImageIgnoringError(image)

			renderProductForm(w, s, form, 0, "", "", err.Error())
			return
		}

		_, err = storage.CreateProductContext(r.Context(), models.Product{
			BarberID:    barber.ID,
			Name:        form.Name,
			Category:    form.Category,
			Description: form.Description,
			Price:       form.Price,
			Quantity:    form.Quantity,
			Image:       image,
			Video:       video,
			IsAvailable: form.Available,
		})

		if err != nil {
			removeImageIgnoringError(image)
			removeVideoIgnoringError(video)
			serverError(w, "create product", err)
			return
		}

		http.Redirect(
			w,
			r,
			"/barber/products",
			http.StatusSeeOther,
		)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func ListProductsHandler(
	w http.ResponseWriter,
	r *http.Request,
	barber models.Barber,
	s sessionData,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	cursor, ok := readPageCursor(w, r)
	if !ok {
		return
	}
	page, err := storage.ProductPage(r.Context(), barber.ID, false, cursor)
	if err != nil {
		serverError(w, "load seller products", err)
		return
	}
	if len(page.Items) == 0 && (cursor.After > 0 || cursor.Before > 0) {
		http.Redirect(w, r, productInventoryPagination(r, cursor, page.Items, false, false).FirstLink, http.StatusSeeOther)
		return
	}
	view := struct {
		Products   []models.Product
		Pagination listPagination
		CSRF       string
	}{
		Products:   page.Items,
		Pagination: productInventoryPagination(r, cursor, page.Items, page.HasPrevious, page.HasNext),
		CSRF:       s.CSRFToken,
	}

	render(w, "barber/products.html", view)
}

func DeleteProductHandler(
	w http.ResponseWriter,
	r *http.Request,
	barber models.Barber,
	s sessionData,
	idSegment string,
) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	if !requireCSRF(w, r, s) {
		return
	}

	id, ok := parseID(idSegment)

	if !ok {
		journeyError(w, "Page not found.", http.StatusNotFound)
		return
	}

	_, err := storage.DeleteProductContext(r.Context(), id, barber.ID)

	switch {
	case errors.Is(err, storage.ErrNotFound),
		errors.Is(err, storage.ErrForbidden):
		journeyError(w, "Page not found.", http.StatusNotFound)
		return

	case err != nil:
		serverError(w, "archive product", err)
		return
	}

	http.Redirect(
		w,
		r,
		"/barber/products",
		http.StatusSeeOther,
	)
}

func ownedProduct(w http.ResponseWriter, r *http.Request, barber models.Barber, idSegment string) (models.Product, bool) {
	id, ok := parseID(idSegment)
	if !ok {
		journeyError(w, "Product not found.", http.StatusNotFound)
		return models.Product{}, false
	}
	product, err := storage.FindProductOwnedBy(r.Context(), id, barber.ID)
	if errors.Is(err, storage.ErrNotFound) || errors.Is(err, storage.ErrForbidden) {
		journeyError(w, "Product not found.", http.StatusNotFound)
		return models.Product{}, false
	}
	if err != nil {
		serverError(w, "find product", err)
		return models.Product{}, false
	}
	return product, true
}

func productFormFromModel(product models.Product) productForm {
	return productForm{Name: product.Name, Category: product.Category, Description: product.Description, Price: product.Price, Quantity: product.Quantity, Available: product.IsAvailable, values: map[string]string{
		"name": product.Name, "category": product.Category, "description": product.Description, "price": formatPrice(product.Price), "quantity": formatInt(product.Quantity),
	}}
}

func ViewProductHandler(w http.ResponseWriter, r *http.Request, barber models.Barber, s sessionData, idSegment string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	product, ok := ownedProduct(w, r, barber, idSegment)
	if !ok {
		return
	}
	renderProductForm(w, s, productFormFromModel(product), product.ID, product.Image, product.Video, "")
}

func EditProductHandler(w http.ResponseWriter, r *http.Request, barber models.Barber, s sessionData, idSegment string) {
	ViewProductHandler(w, r, barber, s, idSegment)
}

func UpdateProductHandler(w http.ResponseWriter, r *http.Request, barber models.Barber, s sessionData, idSegment string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	extendUploadDeadline(w)
	limitFormBody(
		w,
		r,
		maxUploadBytes+maxVideoBytes+maxFormMemory,
	)
	product, ok := ownedProduct(w, r, barber, idSegment)
	if !ok {
		return
	}
	if !requireCSRF(w, r, s) {
		return
	}
	form, problem := readProductForm(r)
	if problem != "" {
		renderProductForm(w, s, form, product.ID, product.Image, product.Video, problem)
		return
	}
	previous := product.Image
	previousVideo := product.Video

	image, err := productImages.save(r, "image")
	switch {
	case errors.Is(err, errNoUpload):
		image = previous
	case err != nil:
		renderProductForm(w, s, form, product.ID, product.Image, product.Video, err.Error())
		return
	}

	video, err := productVideos.save(r, "video")
	switch {
	case errors.Is(err, errNoUpload):
		video = previousVideo
	case err != nil:
		// Only a newly uploaded photo is unreferenced at this point; the
		// stored one must survive the rejected request.
		if image != previous {
			removeImageIgnoringError(image)
		}

		renderProductForm(w, s, form, product.ID, product.Image, product.Video, err.Error())
		return
	}

	uploaded := ""
	if image != previous {
		uploaded = image
	}
	uploadedVideo := ""
	if video != previousVideo {
		uploadedVideo = video
	}
	product.Name = form.Name
	product.Category = form.Category
	product.Description = form.Description
	product.Price = form.Price
	product.Quantity = form.Quantity
	product.IsAvailable = form.Available
	product.Image = image
	product.Video = video
	if err := storage.UpdateProductContext(r.Context(), product); err != nil {
		removeImageIgnoringError(uploaded)
		removeVideoIgnoringError(uploadedVideo)
		serverError(w, "update product", err)
		return
	}
	if uploaded != "" {
		removeImageIgnoringError(previous)
	}
	if uploadedVideo != "" {
		removeVideoIgnoringError(previousVideo)
	}
	http.Redirect(w, r, "/barber/products", http.StatusSeeOther)
}
