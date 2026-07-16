package articles

import (
	"errors"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/schemas/enums"
	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

func CreateArticle(ctrl controllers.ArticleControllerInterface, runTx controllers.TransactionRunner) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		var req schemas.CreateArticleRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}

		if msg := validateArticleInput(req.Title, req.Content, req.URLOriginal, req.Keywords); msg != "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": msg})
		}
		if req.SourceID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "source_id is required"})
		}
		if msg := validateLanguageOriginal(req.LanguageOriginal); msg != "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": msg})
		}

		languageOriginal := req.LanguageOriginal
		var article db.Article
		err := runTx(c.Context(), func(q db.Querier) error {
			var err error
			article, err = ctrl.Create(c.Context(), q, req.Title, req.Content, req.URLOriginal, req.SourceID, req.Keywords, &languageOriginal)
			return err
		})
		if err != nil {
			if errors.Is(err, controllers.ErrArticleAlreadyExists) {
				return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "article with this original URL already exists"})
			}
			if errors.Is(err, controllers.ErrArticleSourceInvalid) {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "source not found or inactive"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create article"})
		}

		response, err := toArticleResponse(article)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to build article response"})
		}

		return c.Status(fiber.StatusCreated).JSON(response)
	}
}

// validateArticleInput enforces the structural rules for an article payload. Returns an
// empty string when valid, or the error message otherwise.
func validateArticleInput(title, content, urlOriginal string, keywords []string) string {
	if strings.TrimSpace(title) == "" {
		return "title is required"
	}
	if strings.TrimSpace(content) == "" {
		return "content is required"
	}
	if urlOriginal == "" {
		return "url_original is required"
	}
	if !isValidURL(urlOriginal) {
		return "url_original must be a valid http or https URL"
	}
	if len(keywords) < schemas.ArticleKeywordsMin {
		return "keywords must have at least 5 items"
	}
	if len(keywords) > schemas.ArticleKeywordsMax {
		return "keywords must have at most 20 items"
	}
	for _, k := range keywords {
		if strings.TrimSpace(k) == "" {
			return "keywords must not contain empty values"
		}
	}
	return ""
}

// validateLanguageOriginal enforces that language_original is present and one of the supported
// language codes. Unlike the CRON (which detects it via lingua-go and may store null), the manual
// create/update endpoints require it. Returns an empty string when valid.
func validateLanguageOriginal(language string) string {
	if language == "" {
		return "language_original is required"
	}
	if !enums.Language(language).IsValid() {
		return "language_original must be a supported language code"
	}
	return ""
}

func isNotFoundError(err error) bool {
	return errors.Is(err, controllers.ErrArticleNotFound)
}

func isValidURL(raw string) bool {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func toArticleResponse(a db.Article) (schemas.ArticleResponse, error) {
	keywords, err := controllers.DecodeKeywords(a.Keywords)
	if err != nil {
		return schemas.ArticleResponse{}, err
	}

	resp := schemas.ArticleResponse{
		ID:          a.ID,
		Status:      a.Status,
		Title:       a.Title,
		Content:     a.Content,
		URLOriginal: a.UrlOriginal,
		Keywords:    keywords,
		SourceID:    a.SourceID,
		CreatedAt:   a.CreatedAt.Time,
		ModifiedAt:  a.ModifiedAt.Ptr(),
	}
	if a.LanguageOriginal.Valid {
		l := a.LanguageOriginal.String
		resp.LanguageOriginal = &l
	}
	return resp, nil
}
