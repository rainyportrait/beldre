package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	"github.com/rainyportrait/beldre/conf"
)

// GetPosts - GET /api/post
func GetPosts(router *gin.RouterGroup, conf *conf.Config) {
	router.GET("/post", func(c *gin.Context) {
		query := c.Query("q")
		page, _ := strconv.Atoi(c.Query("p")) // page is 0 if p cannot become an integer

		pq := newPostsQuery()
		if page > 0 {
			pq.Page = page
		}

		if len(query) != 0 {
			for _, q := range strings.Fields(query) {
				rq := []rune(q)
				if len(rq) != 0 && rq[0] != '-' {
					pq.Tags = append(pq.Tags, q)
				} else {
					pq.Exclude = append(pq.Exclude, string(rq[1:]))
				}
			}
		}

		err := pq.getPosts(conf)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"err": err.Error(),
			})
			return
		}

		err = pq.getTagStats(conf)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"err": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"posts":    pq.Posts,
			"tagStats": pq.TagStats,
		})
	})
}

func newPostsQuery() *postsQuery {
	pq := &postsQuery{}
	pq.Order = "id DESC"
	pq.Limit = 40

	return pq
}

type postsQuery struct {
	Tags    []string
	Exclude []string
	Order   string
	Limit   int
	Page    int

	Posts []struct {
		ID uint64 `json:"id"`

		Source *string `json:"source,omitempty"`
		Hash   string  `json:"hash"`

		CreatedAt string  `db:"created_at" json:"createdAt"`
		UpdatedAt *string `db:"updated_at" json:"updatedAt,omitempty"`
	}

	TagStats []struct {
		ID    uint64 `json:"id"`
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
}

func (pq *postsQuery) getPosts(conf *conf.Config) error {
	sqlStr, err := conf.GetTemplateString("select_posts", &pq)
	if err != nil {
		return err
	}
	args := []interface{}{}

	tc := len(pq.Tags)
	if tc != 0 {
		args = append(args, pq.Tags)
	}

	if len(pq.Exclude) != 0 {
		args = append(args, pq.Exclude)
	}

	if tc != 0 {
		args = append(args, tc)
	}

	args = append(args, pq.Limit, pq.Page)

	q, args, err := sqlx.In(sqlStr, args...)
	if err != nil {
		return err
	}

	err = conf.DB.Select(&pq.Posts, q, args...)
	return err
}

func (pq *postsQuery) getTagStats(conf *conf.Config) error {
	sqlStr, _ := conf.GetTemplateString("select_tag_stats", nil)

	pl := len(pq.Posts)
	if pl == 0 {
		return nil // No post
	}

	postIDs := make([]uint64, pl)
	for i, p := range pq.Posts {
		postIDs[i] = p.ID
	}

	q, args, err := sqlx.In(sqlStr, postIDs)
	if err != nil {
		return err
	}

	err = conf.DB.Select(&pq.TagStats, q, args...)
	if err != nil {
		return err
	}

	return nil
}
