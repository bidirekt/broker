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

	return parsePage(string(query.Peek(param)))
}

func parsePage(raw string) (page int64, ok bool) {
	page, ok = ParsePositiveInteger(raw)

	return page, ok && page <= math.MaxInt64/PageSize
}

func Offset(page int64) int {
	return OffsetOfSize(page, PageSize)
}

func OffsetOfSize(page int64, size int) int {
	return int(page-1) * size
}

func NewPager(ctx fiber.Ctx, param string, page int64, total int) (pager Pager, ok bool) {
	return NewPagerOfSize(ctx, param, page, total, PageSize)
}

func NewPagerOfSize(ctx fiber.Ctx, param string, page int64, total, size int) (pager Pager, ok bool) {
	lastPage := max(1, int64((total+size-1)/size))
	if page > lastPage {
		return Pager{}, false
	}

	pager = Pager{Page: page, Last: min(OffsetOfSize(page, size)+size, total), Total: total}
	if total > 0 {
		pager.First = OffsetOfSize(page, size) + 1
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

	return parsePage(string(args.Peek(param)))
}
