package crawler

import (
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/blake2s"
)

var pagesem chan bool = make(chan bool, 10)
var imgsem chan bool = make(chan bool, 15)

var client http.Client = http.Client{
	Timeout: time.Duration(20 * time.Second),
}

var back *backoff.ExponentialBackOff = backoff.NewExponentialBackOff()

// CrawlRule - Crawls a tag in rule34.xxx
func CrawlRule(tag string, db *sqlx.DB) {
	storeFullTag(tag, db)
	waitForChannels()
}

func waitForChannels() {
	for i := 0; i < cap(pagesem); i++ {
		pagesem <- true
	}

	for i := 0; i < cap(imgsem); i++ {
		imgsem <- true
	}

	for i := 0; i < cap(pagesem); i++ {
		<-pagesem
	}

	for i := 0; i < cap(imgsem); i++ {
		<-imgsem
	}
}

func storeFullTag(tag string, db *sqlx.DB) {
	url := fmt.Sprintf("https://rule34.xxx/index.php?page=dapi&s=post&q=index&tags=%s", tag)
	p, err := getPosts(url)
	if err != nil {
		log.Error(err.Error())
		return
	}

	pagesem <- true
	go func(p *postsRequest, db *sqlx.DB) {
		storePageFromStruct(p.Posts, url, db)
		<-pagesem
	}(p, db)

	pages := p.Count / 100
	for i := 1; i < pages; i++ {
		pagesem <- true
		go func(url string, i int, db *sqlx.DB) {
			storePageFromURL(fmt.Sprintf("%s&pid=%d", url, i), db)
			<-pagesem
		}(url, i, db)
	}
}

func storeTagPage(tag string, page int, db *sqlx.DB) {
	url := fmt.Sprintf("https://rule34.xxx/index.php?page=dapi&s=post&q=index&tags=%s&pid=%d", tag, page)
	storePageFromURL(url, db)
}

func getPosts(url string) (*postsRequest, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	rawBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}

	p := &postsRequest{}
	err = xml.Unmarshal(rawBody, p)
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}

	return p, nil
}

func storePageFromURL(url string, db *sqlx.DB) {
	p, _ := getPosts(url)
	storePageFromStruct(p.Posts, url, db)
}

func storePageFromStruct(p []post, url string, db *sqlx.DB) {
	for _, i := range p {
		imgsem <- true
		go func(p post, url string, db *sqlx.DB) {
			storePost(p, url, db)
			<-imgsem
		}(i, url, db)
	}
}

func storePost(p post, url string, db *sqlx.DB) {
	row, err := db.Query(
		"SELECT url FROM post_crawl_info WHERE url = ?",
		p.FileURL,
	)
	if err != nil {
		log.Error(err.Error())
		return
	}
	defer row.Close()

	if row.Next() {
		// Already exists within database
		return
	}

	ext := getImageExtensionFromURL(p.FileURL)
	hash, err := storeImage(p.FileURL, ext)
	if err != nil {
		log.Error(err.Error())
		return
	}

	dataStr, err := json.Marshal(p)
	if err != nil {
		log.Error(err.Error())
		return
	}

	tx, err := db.Beginx()
	if err != nil {
		log.Error(err.Error())
		return
	}

	res, err := tx.Exec(
		`INSERT INTO post_crawl_info (site, url, data)
		VALUES (?, ?, ?)`,
		"rule34.xxx",
		p.FileURL,
		string(dataStr),
	)
	if err != nil {
		tx.Rollback()
		log.Error(err.Error())
		return
	}

	crawlInfoID, err := res.LastInsertId()
	if err != nil {
		tx.Rollback()
		log.Error(err.Error())
		return
	}

	if len(*p.Source) == 0 {
		p.Source = nil
	}

	res, err = tx.Exec(
		`INSERT INTO post (uploader, hash, crawl_info, source)
		VALUES (
			(SELECT id FROM user WHERE name = 'crawler'),
			?,
			?,
			?
		)
		ON DUPLICATE KEY UPDATE id = id`,
		hash,
		crawlInfoID,
		p.Source,
	)
	if err != nil {
		tx.Rollback()
		log.WithFields(log.Fields{
			"url": p.FileURL,
		}).Error(err.Error())
		return
	}

	tags := strings.Split(p.Tags, " ")
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if len(tag) > 0 {
			_, err := tx.Exec(
				`INSERT INTO tag (name)
				VALUES (?)
				ON DUPLICATE KEY UPDATE id = id`,
				tag,
			)
			if err != nil {
				tx.Rollback()
				log.Error(err.Error())
				return
			}

			_, err = tx.Exec(
				`INSERT INTO post_tag (post, tag, assigned_by)
				VALUES (
					(SELECT id FROM post WHERE hash = ?), 
					(SELECT id FROM tag WHERE name = ?),
					(SELECT id FROM user WHERE name = 'crawler')
				) 
				ON DUPLICATE KEY UPDATE id = id`,
				hash,
				tag,
			)
			if err != nil {
				tx.Rollback()
				log.Error(err.Error())
				return
			}
		}
	}

	tx.Commit()
}

func storeImage(url string, ext string) (hash string, err error) {
	o := func() error {
		resp, err := http.Get(url)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("%s responded with %d a non 200 status code", url, resp.StatusCode)
		}

		f, err := ioutil.TempFile("D:/images", "tmp-")
		if err != nil {
			return backoff.Permanent(errors.New("creating a tmp file failed"))
		}

		h, err := blake2s.New256(nil)
		if err != nil {
			os.Remove(f.Name())
			return backoff.Permanent(err)
		}

		tee := io.TeeReader(resp.Body, h)
		_, err = io.Copy(f, tee)
		if err != nil {
			os.Remove(f.Name())
			return err
		}
		f.Close()

		hash = base64.URLEncoding.EncodeToString(h.Sum(nil))
		err = os.Rename(f.Name(), fmt.Sprintf("D:/images/%s.%s", hash, ext))
		if err != nil {
			os.Remove(f.Name())
			return backoff.Permanent(err)
		}

		return nil
	}

	err = backoff.Retry(o, back)
	return
}

func getImageExtensionFromURL(url string) string {
	return string([]rune(url)[strings.LastIndex(url, ".")+1 : len(url)])
}

type postsRequest struct {
	XMLName xml.Name `xml:"posts"`
	Count   int      `xml:"count,attr"`
	Offset  int      `xml:"offset,attr"`
	Posts   []post   `xml:"post"`
}

type post struct {
	Height        int     `xml:"height,attr" json:"height"`
	Score         int     `xml:"score,attr" json:"score"`
	FileURL       string  `xml:"file_url,attr" json:"file_url"`
	ParentID      int     `xml:"parent_id,attr" json:"parent_id"`
	SampleURL     string  `xml:"sample_url,attr" json:"sample_url"`
	SampleWidth   string  `xml:"sample_width,attr" json:"sample_width"`
	SampleHeight  string  `xml:"sample_height,attr" json:"sample_height"`
	PreviewURL    string  `xml:"preview_url,attr" json:"preview_url"`
	Rating        string  `xml:"rating,attr" json:"rating"`
	Tags          string  `xml:"tags,attr" json:"tags"`
	ID            int     `xml:"id,attr" json:"id"`
	Width         int     `xml:"width,attr" json:"width"`
	Change        string  `xml:"change,attr" json:"change"`
	Md5           string  `xml:"md5,attr" json:"md5"`
	CreatorID     int     `xml:"creator_id,attr" json:"creator_id"`
	HasChildren   bool    `xml:"has_children,attr" json:"has_children"`
	CreatedAt     string  `xml:"created_at,attr" json:"created_at"`
	Status        string  `xml:"status,attr" json:"status"`
	Source        *string `xml:"source,attr" json:"source"`
	HasNotes      bool    `xml:"has_notes,attr" json:"has_notes"`
	HasComments   bool    `xml:"has_comments,attr" json:"has_comments"`
	PreviewWidth  int     `xml:"preview_width,attr" json:"preview_width"`
	PreviewHeight int     `xml:"preview_height,attr" json:"preview_height"`
}
