package pages

import (
	"math"
	"net/url"
	"strconv"

	"github.com/gofiber/fiber/v3"
)

const PageSize = 25

type Pager struct {
	Page        int64
	First       int
	Last        int
	Total       int
	PreviousURL string
	NextURL     string
}

func RequestedPage(ctx fiber.Ctx, param string) (page int64, ok bool) {
	query := ctx.Request().URI().QueryArgs()
	if !query.Has(param) {
		return 1, true
	}

	page, ok = ParsePositiveInteger(string(query.Peek(param)))

	return page, ok && page <= math.MaxInt64/PageSize
}

func Offset(page int64) int {
	return int(page-1) * PageSize
}

func NewPager(ctx fiber.Ctx, param string, page int64, total int) (pager Pager, ok bool) {
	lastPage := max(1, int64((total+PageSize-1)/PageSize))
	if page > lastPage {
		return Pager{}, false
	}

	pager = Pager{Page: page, Last: min(Offset(page)+PageSize, total), Total: total}
	if total > 0 {
		pager.First = Offset(page) + 1
	}
	if page > 1 {
		pager.PreviousURL = pageURL(ctx, param, page-1)
	}
	if page < lastPage {
		pager.NextURL = pageURL(ctx, param, page+1)
	}

	return pager, true
}

func pageURL(ctx fiber.Ctx, param string, page int64) string {
	query, _ := url.ParseQuery(string(ctx.Request().URI().QueryString()))
	query.Set(param, strconv.FormatInt(page, 10))

	return string(ctx.Request().URI().PathOriginal()) + "?" + query.Encode()
}

func ReturnPage(ctx fiber.Ctx, param string) (page int64, ok bool) {
	args := ctx.Request().URI().QueryArgs()
	if ctx.Method() == fiber.MethodPost {
		args = ctx.Request().PostArgs()
	}
	if !args.Has(param) {
		return 0, true
	}

	return ParsePositiveInteger(string(args.Peek(param)))
}
