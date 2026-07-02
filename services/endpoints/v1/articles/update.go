package articles

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

func UpdateArticle(ctrl controllers.ArticleControllerInterface, runTx controllers.TransactionRunner) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		id := c.Params("id")
		if id == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id is required"})
		}

		var req schemas.UpdateArticleRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}

		if msg := validateArticleInput(req.Title, req.Content, req.URLOriginal, req.Keywords); msg != "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": msg})
		}
		if msg := validateLanguageOriginal(req.LanguageOriginal); msg != "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": msg})
		}

		languageOriginal := req.LanguageOriginal
		var article db.Article
		err := runTx(c.Context(), func(q db.Querier) error {
			var err error
			article, err = ctrl.Update(c.Context(), q, id, req.Title, req.Content, req.URLOriginal, req.Keywords, &languageOriginal)
			return err
		})
		if err != nil {
			if errors.Is(err, controllers.ErrArticleNotFound) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "article not found"})
			}
			if errors.Is(err, controllers.ErrArticleAlreadyExists) {
				return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "article with this original URL already exists"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update article"})
		}

		response, err := toArticleResponse(article)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to build article response"})
		}

		return c.JSON(response)
	}
}
