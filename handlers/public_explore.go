package handlers

import (
	"net/http"
	"strings"
	"barber-booking/models"
	"barber-booking/storage"
)

type exploreCard struct{ models.Barber; Hairstyles int; FromPrice float64; Rating float64; ReviewCount int }
type exploreView struct{ Items []exploreCard; States []string; SelectedState,SelectedArea string }
type publicBarberView struct{ Barber models.Barber; Hairstyles []models.Hairstyle; Reviews []models.Review; Rating float64; ReviewCount int }

func ExploreRouter(w http.ResponseWriter,r *http.Request){
	parts:=pathParts(r.URL.Path)
	if len(parts)==1 { ExploreBarbersHandler(w,r); return }
	if len(parts)==2 { ExploreBarberHandler(w,r,parts[1]); return }
	http.NotFound(w,r)
}

func ExploreBarbersHandler(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodGet{methodNotAllowed(w,http.MethodGet);return}
	all:=storage.ListActiveBarbers();state:=strings.TrimSpace(r.URL.Query().Get("state"));area:=strings.TrimSpace(r.URL.Query().Get("area"))
	seen:=map[string]bool{};var states []string;var cards []exploreCard
	for _,b:=range all{
		if b.State!=""&&!seen[strings.ToLower(b.State)]{states=append(states,b.State);seen[strings.ToLower(b.State)]=true}
		if state==""||!strings.EqualFold(b.State,state)||(area!=""&&!strings.EqualFold(b.LocalArea,area)){continue}
		b.PasswordHash="";card:=exploreCard{Barber:b}
		for _,style:=range storage.ListAvailableHairstylesByBarber(b.ID){card.Hairstyles++;if card.Hairstyles==1||style.Price<card.FromPrice{card.FromPrice=style.Price}}
		card.Rating,card.ReviewCount=storage.ReviewSummary(b.ID);cards=append(cards,card)
	}
	render(w,"explore.html",exploreView{Items:cards,States:states,SelectedState:state,SelectedArea:area})
}

func ExploreBarberHandler(w http.ResponseWriter,r *http.Request,idSegment string){
	if r.Method!=http.MethodGet{methodNotAllowed(w,http.MethodGet);return}
	id,ok:=parseID(idSegment);if !ok{http.NotFound(w,r);return}
	b,found:=storage.FindBarberByID(id);if !found||!b.IsActive(){http.NotFound(w,r);return};b.PasswordHash=""
	rating,count:=storage.ReviewSummary(id)
	render(w,"explore-barber.html",publicBarberView{Barber:b,Hairstyles:storage.ListAvailableHairstylesByBarber(id),Reviews:storage.ListReviewsByBarber(id),Rating:rating,ReviewCount:count})
}
