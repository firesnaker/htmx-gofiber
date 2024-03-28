package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"                 //we'll use fiber framework
	"github.com/gofiber/fiber/v2/log"             //logging
	"github.com/gofiber/fiber/v2/middleware/cors" //middleware to handle cors
	"github.com/gofiber/template/html/v2"         //templating
	"github.com/joho/godotenv"                    //we'll use godotenv to handle env variable
	"github.com/mtslzr/pokeapi-go"                //connect to 3rd party pokeapi
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatData struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float32   `json:"temperature"`
}

type ResponseData struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

var (
	conversation []Message
)

func main() {
	//loading godot to read env
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	engine := html.New("./templates", ".html")

	app := fiber.New(fiber.Config{
		Views: engine,
	})

	app.Use(cors.New())

	app.Static("/css", "./css")

	app.Get("/", func(c *fiber.Ctx) error {
		l, _ := pokeapi.Resource("pokemon")

		for i, p := range l.Results {
			l.Results[i].Name = capitalize(p.Name)
		}

		return c.Render("index", fiber.Map{
			"Results": l.Results,
		})
	})

	app.Get("/pokemon/:name", func(c *fiber.Ctx) error {
		name := strings.ToLower(c.Params("name"))
		p, _ := pokeapi.Pokemon(name)

		for i, a := range p.Abilities {
			p.Abilities[i].Ability.Name = capitalize(a.Ability.Name)
		}

		// Reset the conversation
		conversation = []Message{}

		// Append the system message to the conversation
		conversation = append(conversation, Message{Role: "system", Content: "You are a caring and knowledgeable " + p.Name + " speaking to your trainer."})

		return c.Render("pokemon-detail", fiber.Map{
			"ImageUrl":  p.Sprites.FrontDefault,
			"Name":      capitalize(p.Name),
			"Height":    p.Height,
			"Weight":    p.Weight,
			"Hp":        p.Stats[0].BaseStat,
			"Abilities": p.Abilities,
			"Types":     p.Types,
		})
	})

	app.Get("/search", func(c *fiber.Ctx) error {
		q := strings.ToLower(c.Query("q"))
		p, err := pokeapi.Pokemon(q)

		if err != nil {
			return c.Render("pokemon-detail", fiber.Map{
				"Error": "Oops! There seems to be no Pokemon by that name in our Pokedex.",
			})
		}

		for i, a := range p.Abilities {
			p.Abilities[i].Ability.Name = capitalize(a.Ability.Name)
		}

		// Reset the conversation
		conversation = []Message{}

		// Append the system message to the conversation
		conversation = append(conversation, Message{Role: "system", Content: "You are a caring and knowledgeable " + p.Name + " speaking to your trainer."})

		return c.Render("pokemon-detail", fiber.Map{
			"ImageUrl":  p.Sprites.FrontDefault,
			"Name":      capitalize(p.Name),
			"Height":    p.Height,
			"Weight":    p.Weight,
			"Hp":        p.Stats[0].BaseStat,
			"Abilities": p.Abilities,
			"Types":     p.Types,
		})
	})

	// Handle chat request
	app.Post("/chat", func(c *fiber.Ctx) error {
		text := c.FormValue("message")

		conversation = append(conversation, Message{Role: "user", Content: text})

		data := ChatData{"gpt-3.5-turbo", conversation, 0.7} //you can change the AI model (gpt-3.5-turbo) and temperature here (0.7)

		payloadBuf := new(bytes.Buffer)
		json.NewEncoder(payloadBuf).Encode(data)

		req, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", payloadBuf)

		req.Header.Add("Authorization", "Bearer "+os.Getenv("OPENAI_API_KEY"))
		req.Header.Add("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)

		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		var responseData ResponseData
		json.Unmarshal(body, &responseData)

		conversation = append(conversation, Message{Role: "assistant", Content: responseData.Choices[0].Message.Content})

		// Create HTML string based on conversation data.
		var htmlStr string
		for _, msg := range conversation {
			if msg.Role == "user" {
				htmlStr += fmt.Sprintf("<div class='w-full rounded p-2 mb-2 bg-green-200 text-green-900 text-right'>%s</div>", msg.Content)
			} else if msg.Role == "assistant" {
				htmlStr += fmt.Sprintf("<div class='w-full rounded p-2 mb-2 bg-blue-200 text-blue-900'>%s</div>", msg.Content)
			}
		}

		c.Set("Content-Type", "text/html") // set the content type to HTML
		return c.SendString(htmlStr)       // send HTML string
	})

	log.Fatal(app.Listen(":3000"))
}

func capitalize(s string) string {
	if len(s) == 0 {
		return ""
	}
	return strings.ToUpper(string(s[0])) + strings.ToLower(s[1:])
}
