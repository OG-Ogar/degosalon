package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"barber-booking/models"
)

const (
	socialFeedLimit       = 30
	socialStatusLimit     = 20
	socialPostMaxRunes    = 2000
	socialCommentMaxRunes = 500
)

var ErrInvalidSocialPost = errors.New("invalid social post")
var ErrInvalidSocialComment = errors.New("invalid social comment")

type socialLikeKey struct {
	postID int
	role   string
	actor  int
}

// CreateSocialPost stores a post for an authenticated customer or active barber.
func CreateSocialPost(ctx context.Context, post models.SocialPost) (models.SocialPost, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	post.Text = strings.TrimSpace(post.Text)
	post.BackgroundColor = strings.ToLower(strings.TrimSpace(post.BackgroundColor))
	post.TextStyle = strings.TrimSpace(post.TextStyle)
	if !validSocialActor(post.AuthorRole, post.AuthorID) || utf8.RuneCountInString(post.Text) > socialPostMaxRunes || !validSocialAppearance(post.BackgroundColor, post.TextStyle) || (post.Image != "" && post.Video != "") || (post.Text == "" && post.Image == "" && post.Video == "") {
		return models.SocialPost{}, ErrInvalidSocialPost
	}
	if err := ctx.Err(); err != nil {
		return models.SocialPost{}, err
	}
	if databaseEnabled() {
		var query string
		switch post.AuthorRole {
		case models.SenderCustomer:
			query = `insert into public.social_posts(author_role,customer_id,body,background_color,text_style,image,video)
select 'customer',id,$2,$3,$4,$5,$6 from public.customers where id=$1 returning id,created_at`
		case models.SenderBarber:
			query = `insert into public.social_posts(author_role,barber_id,body,background_color,text_style,image,video)
select 'barber',id,$2,$3,$4,$5,$6 from public.barbers where id=$1 and status='Active' returning id,created_at`
		}
		err := db.QueryRowContext(ctx, query, post.AuthorID, post.Text, post.BackgroundColor, post.TextStyle, post.Image, post.Video).Scan(&post.ID, &post.CreatedAt)
		if errors.Is(err, sql.ErrNoRows) {
			return models.SocialPost{}, ErrNotFound
		}
		return post, err
	}
	mu.Lock()
	defer mu.Unlock()
	if err := ctx.Err(); err != nil {
		return models.SocialPost{}, err
	}
	if !socialActorExistsLocked(post.AuthorRole, post.AuthorID) {
		return models.SocialPost{}, ErrNotFound
	}
	if err := guardMediaLocked(post.Image); err != nil {
		return models.SocialPost{}, err
	}
	if err := guardMediaLocked(post.Video); err != nil {
		return models.SocialPost{}, err
	}
	post.ID = nextID(&lastSocialPostID)
	post.CreatedAt = time.Now().UTC()
	socialPosts = append(socialPosts, post)
	return post, nil
}

// HomeFeed returns the newest shared posts, personalized with this viewer's likes.
func HomeFeed(ctx context.Context, role string, actorID int) ([]models.SocialPost, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if !validSocialActor(role, actorID) {
		return nil, ErrNotFound
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if databaseEnabled() {
		query := `select p.id,p.author_role,p.customer_id,p.barber_id,
 coalesce(c.full_name,b.full_name,''),coalesce(nullif(c.profile_image,''),nullif(b.personal_image,''),b.profile_image,''),
 p.body,p.background_color,p.text_style,p.image,p.video,p.created_at,
 (select count(*) from public.social_post_likes l where l.post_id=p.id),
 exists(select 1 from public.social_post_likes l where l.post_id=p.id and (($1='customer' and l.customer_id=$2) or ($1='barber' and l.barber_id=$2))),
 coalesce((select json_agg(q) from (
   select cm.id,cm.post_id,cm.author_role,cm.customer_id,cm.barber_id,
    coalesce(cc.full_name,cb.full_name,'' ) as author_name,
    coalesce(nullif(cc.profile_image,''),nullif(cb.personal_image,''),cb.profile_image,'') as author_image,
    cm.body as text,cm.created_at
   from public.social_post_comments cm
   left join public.customers cc on cc.id=cm.customer_id
   left join public.barbers cb on cb.id=cm.barber_id
   where cm.post_id=p.id order by cm.created_at desc,cm.id desc limit 3
 ) q),'[]'::json)
 from public.social_posts p
 left join public.customers c on c.id=p.customer_id
 left join public.barbers b on b.id=p.barber_id
 where p.author_role='customer' or b.status='Active'
 order by p.created_at desc,p.id desc limit $3`
		return queryRecords(ctx, query, func(rows *sql.Rows) (models.SocialPost, error) {
			var post models.SocialPost
			var customerID, barberID sql.NullInt64
			var comments []byte
			err := rows.Scan(&post.ID, &post.AuthorRole, &customerID, &barberID, &post.AuthorName, &post.AuthorImage,
				&post.Text, &post.BackgroundColor, &post.TextStyle, &post.Image, &post.Video, &post.CreatedAt,
				&post.LikeCount, &post.LikedByMe, &comments)
			if err != nil {
				return post, err
			}
			if customerID.Valid {
				post.AuthorID = int(customerID.Int64)
			} else if barberID.Valid {
				post.AuthorID = int(barberID.Int64)
			}
			if err := json.Unmarshal(comments, &post.Comments); err != nil {
				return post, err
			}
			return post, nil
		}, role, actorID, socialFeedLimit)
	}
	mu.RLock()
	defer mu.RUnlock()
	posts := make([]models.SocialPost, 0, len(socialPosts))
	for _, post := range socialPosts {
		if !socialActorExistsLocked(post.AuthorRole, post.AuthorID) {
			continue
		}
		post.LikeCount = 0
		post.LikedByMe = socialLikes[socialLikeKey{post.ID, role, actorID}]
		for key := range socialLikes {
			if key.postID == post.ID {
				post.LikeCount++
			}
		}
		post.Comments = nil
		for i := len(socialComments) - 1; i >= 0 && len(post.Comments) < 3; i-- {
			if socialComments[i].PostID == post.ID {
				post.Comments = append(post.Comments, socialComments[i])
			}
		}
		posts = append(posts, post)
	}
	sort.Slice(posts, func(i, j int) bool {
		if posts[i].CreatedAt.Equal(posts[j].CreatedAt) {
			return posts[i].ID > posts[j].ID
		}
		return posts[i].CreatedAt.After(posts[j].CreatedAt)
	})
	if len(posts) > socialFeedLimit {
		posts = posts[:socialFeedLimit]
	}
	return posts, nil
}

// HairstyleStatuses returns active barbers' currently bookable styles for the status row.
func HairstyleStatuses(ctx context.Context) ([]models.HairstyleStatus, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if databaseEnabled() {
		return queryRecords(ctx, `select h.id,b.id,b.full_name,coalesce(nullif(b.personal_image,''),b.profile_image),b.shop_name,h.name,h.image,h.video
from public.hairstyles h join public.barbers b on b.id=h.barber_id
where b.status='Active' and h.is_available and not h.is_archived
order by h.id desc limit $1`, func(rows *sql.Rows) (models.HairstyleStatus, error) {
			var v models.HairstyleStatus
			err := rows.Scan(&v.HairstyleID, &v.BarberID, &v.BarberName, &v.BarberImage, &v.ShopName, &v.Name, &v.Image, &v.Video)
			return v, err
		}, socialStatusLimit)
	}
	mu.RLock()
	defer mu.RUnlock()
	barberByID := make(map[int]models.Barber, len(barbers))
	for _, barber := range barbers {
		if barber.IsActive() {
			barberByID[barber.ID] = barber
		}
	}
	var statuses []models.HairstyleStatus
	for i := len(hairstyles) - 1; i >= 0 && len(statuses) < socialStatusLimit; i-- {
		style := hairstyles[i]
		barber, ok := barberByID[style.BarberID]
		if !ok || !style.IsAvailable || style.Archived {
			continue
		}
		photo := barber.PersonalImage
		if photo == "" {
			photo = barber.ProfileImage
		}
		statuses = append(statuses, models.HairstyleStatus{HairstyleID: style.ID, BarberID: barber.ID, BarberName: barber.FullName, BarberImage: photo, ShopName: barber.ShopName, Name: style.Name, Image: style.Image, Video: style.Video})
	}
	return statuses, nil
}

// DeleteSocialPost removes one post only when the signed-in account wrote it.
// The returned post carries the media URLs so the caller can remove the files.
func DeleteSocialPost(ctx context.Context, postID int, role string, actorID int) (models.SocialPost, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if postID <= 0 || !validSocialActor(role, actorID) {
		return models.SocialPost{}, ErrNotFound
	}
	if err := ctx.Err(); err != nil {
		return models.SocialPost{}, err
	}
	if databaseEnabled() {
		var query string
		if role == models.SenderCustomer {
			query = `delete from public.social_posts where id=$1 and author_role='customer' and customer_id=$2 returning id,coalesce(image,''),coalesce(video,'')`
		} else {
			query = `delete from public.social_posts where id=$1 and author_role='barber' and barber_id=$2 returning id,coalesce(image,''),coalesce(video,'')`
		}
		var post models.SocialPost
		if err := db.QueryRowContext(ctx, query, postID, actorID).Scan(&post.ID, &post.Image, &post.Video); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return models.SocialPost{}, ErrNotFound
			}
			return models.SocialPost{}, err
		}
		post.AuthorRole = role
		post.AuthorID = actorID
		return post, nil
	}
	mu.Lock()
	defer mu.Unlock()
	if err := ctx.Err(); err != nil {
		return models.SocialPost{}, err
	}
	for i, post := range socialPosts {
		if post.ID != postID {
			continue
		}
		if post.AuthorRole != role || post.AuthorID != actorID {
			return models.SocialPost{}, ErrNotFound
		}
		socialPosts = append(socialPosts[:i], socialPosts[i+1:]...)
		for key := range socialLikes {
			if key.postID == postID {
				delete(socialLikes, key)
			}
		}
		comments := socialComments[:0]
		for _, comment := range socialComments {
			if comment.PostID != postID {
				comments = append(comments, comment)
			}
		}
		socialComments = comments
		return post, nil
	}
	return models.SocialPost{}, ErrNotFound
}

// SetSocialPostLiked sets the like state idempotently for one authenticated account.
func SetSocialPostLiked(ctx context.Context, postID int, role string, actorID int, liked bool) error {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if postID <= 0 || !validSocialActor(role, actorID) {
		return ErrNotFound
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if databaseEnabled() {
		var exists bool
		if err := db.QueryRowContext(ctx, "select exists(select 1 from public.social_posts where id=$1)", postID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		if liked {
			if role == models.SenderCustomer {
				_, err := db.ExecContext(ctx, "insert into public.social_post_likes(post_id,customer_id) values($1,$2) on conflict(post_id,customer_id) where customer_id is not null do nothing", postID, actorID)
				return err
			}
			_, err := db.ExecContext(ctx, "insert into public.social_post_likes(post_id,barber_id) values($1,$2) on conflict(post_id,barber_id) where barber_id is not null do nothing", postID, actorID)
			return err
		}
		if role == models.SenderCustomer {
			_, err := db.ExecContext(ctx, "delete from public.social_post_likes where post_id=$1 and customer_id=$2", postID, actorID)
			return err
		}
		_, err := db.ExecContext(ctx, "delete from public.social_post_likes where post_id=$1 and barber_id=$2", postID, actorID)
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	found := false
	for _, post := range socialPosts {
		if post.ID == postID {
			found = true
			break
		}
	}
	if !found {
		return ErrNotFound
	}
	if socialLikes == nil {
		socialLikes = make(map[socialLikeKey]bool)
	}
	key := socialLikeKey{postID, role, actorID}
	if liked {
		socialLikes[key] = true
	} else {
		delete(socialLikes, key)
	}
	return nil
}

// AddSocialComment appends a plain-text comment using the authenticated author's identity.
func AddSocialComment(ctx context.Context, comment models.SocialComment) error {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	comment.Text = strings.TrimSpace(comment.Text)
	if comment.PostID <= 0 || !validSocialActor(comment.AuthorRole, comment.AuthorID) || comment.Text == "" || utf8.RuneCountInString(comment.Text) > socialCommentMaxRunes {
		return ErrInvalidSocialComment
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if databaseEnabled() {
		var query string
		if comment.AuthorRole == models.SenderCustomer {
			query = `insert into public.social_post_comments(post_id,author_role,customer_id,body) select $1,'customer',id,$3 from public.customers where id=$2 and exists(select 1 from public.social_posts where id=$1)`
		} else {
			query = `insert into public.social_post_comments(post_id,author_role,barber_id,body) select $1,'barber',id,$3 from public.barbers where id=$2 and status='Active' and exists(select 1 from public.social_posts where id=$1)`
		}
		result, err := db.ExecContext(ctx, query, comment.PostID, comment.AuthorID, comment.Text)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if !socialActorExistsLocked(comment.AuthorRole, comment.AuthorID) {
		return ErrNotFound
	}
	found := false
	for _, post := range socialPosts {
		if post.ID == comment.PostID {
			found = true
			break
		}
	}
	if !found {
		return ErrNotFound
	}
	comment.ID = nextID(&lastSocialCommentID)
	comment.CreatedAt = time.Now().UTC()
	socialComments = append(socialComments, comment)
	return nil
}

func validSocialActor(role string, id int) bool {
	return id > 0 && (role == models.SenderCustomer || role == models.SenderBarber)
}
func validSocialAppearance(color, style string) bool {
	colors := map[string]bool{"#fff0c2": true, "#e8f1ff": true, "#e8f7ec": true, "#fae8ef": true, "#eee9ff": true, "#ffffff": true}
	return colors[color] && (style == "normal" || style == "bold" || style == "italic")
}
func socialActorExistsLocked(role string, id int) bool {
	if role == models.SenderCustomer {
		_, ok := findCustomerByID(id)
		return ok
	}
	for _, barber := range barbers {
		if barber.ID == id {
			return barber.IsActive()
		}
	}
	return false
}
