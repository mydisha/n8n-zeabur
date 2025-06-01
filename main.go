package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"

	_ "github.com/mattn/go-sqlite3"
	"google.golang.org/protobuf/proto"
)

type ExpenseData struct {
	Item        string    `json:"item"`
	Amount      float64   `json:"amount"`
	Category    string    `json:"category"`
	GroupName   string    `json:"group_name"`
	SenderName  string    `json:"sender_name"`
	SenderPhone string    `json:"sender_phone"`
	Timestamp   time.Time `json:"timestamp"`
	MessageID   string    `json:"message_id"`
}

var (
	client        *whatsmeow.Client
	n8nWebhookURL = os.Getenv("N8N_WEBHOOK_URL")
	llmProvider   = os.Getenv("LLM_PROVIDER")
	llmAPIKey     = os.Getenv("LLM_API_KEY")
	
	// Regex patterns
	expenseRegex  = regexp.MustCompile(`^(.+?)\s+(\d+(?:[.,]\d{3})*(?:[.,]\d{2})?)$`)
	commandRegex  = regexp.MustCompile(`^/(summary|categories|help|status)\s*(.*)$`)
	
	// Common categories for quick matching
	categoryKeywords = map[string]string{
		"food": "Food", "makan": "Food", "nasi": "Food", "ayam": "Food", "sate": "Food",
		"transport": "Transportation", "grab": "Transportation", "gojek": "Transportation", "taxi": "Transportation",
		"shopping": "Shopping", "beli": "Shopping", "belanja": "Shopping",
		"bill": "Bills", "listrik": "Bills", "air": "Bills", "internet": "Bills",
		"health": "Health", "obat": "Health", "dokter": "Health", "rumah sakit": "Health",
	}
)

func main() {
	// Initialize WhatsApp client
	if err := initWhatsApp(); err != nil {
		log.Fatal("❌ Failed to initialize WhatsApp:", err)
	}

	// Setup HTTP server
	setupHTTPServer()
	
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	
	log.Printf("🚀 WhatsApp Expense Bot started on port %s", port)
	log.Printf("📡 N8N Webhook: %s", maskURL(n8nWebhookURL))
	log.Printf("🤖 LLM Provider: %s", llmProvider)
	
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("❌ Failed to start server:", err)
	}
}

func initWhatsApp() error {
	dbLog := waLog.Stdout("Database", "INFO", true)
	container, err := sqlstore.New("sqlite3", "file:whatsapp.db?_foreign_keys=on", dbLog)
	if err != nil {
		return fmt.Errorf("failed to create database: %v", err)
	}

	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		return fmt.Errorf("failed to get device: %v", err)
	}

	clientLog := waLog.Stdout("Client", "INFO", true)
	client = whatsmeow.NewClient(deviceStore, clientLog)
	
	// Add event handler
	client.AddEventHandler(handleMessage)

	// Connect to WhatsApp
	if client.Store.ID == nil {
		// No existing session, need to pair
		qrChan, _ := client.GetQRChannel(context.Background())
		err = client.Connect()
		if err != nil {
			return fmt.Errorf("failed to connect: %v", err)
		}

		// Wait for QR code
		for evt := range qrChan {
			if evt.Event == "code" {
				log.Printf("🔗 QR Code: %s", evt.Code)
				log.Println("📱 Scan this QR code with your WhatsApp mobile app")
			} else {
				log.Printf("📱 Login event: %s", evt.Event)
				if evt.Event == "success" {
					break
				}
			}
		}
	} else {
		// Existing session, just connect
		err = client.Connect()
		if err != nil {
			return fmt.Errorf("failed to connect: %v", err)
		}
	}

	log.Println("✅ WhatsApp connected successfully!")
	return nil
}

func setupHTTPServer() {
	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status":    "healthy",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	})

	// QR Code endpoint for pairing
	http.HandleFunc("/qr", func(w http.ResponseWriter, r *http.Request) {
		if client.Store.ID != nil {
			w.Write([]byte("Already logged in"))
			return
		}
		
		qrChan, _ := client.GetQRChannel(context.Background())
		for evt := range qrChan {
			if evt.Event == "code" {
				w.Write([]byte(fmt.Sprintf("QR Code: %s\nScan with WhatsApp mobile app", evt.Code)))
				return
			}
		}
	})
}

func handleMessage(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		// Only process group messages that are not from us
		if !v.Info.IsGroup || v.Info.IsFromMe {
			return
		}
		
		messageText := getMessageText(v)
		if messageText == "" {
			return
		}
		
		log.Printf("📨 Message from %s in %s: %s", v.Info.PushName, v.Info.Chat, messageText)
		
		// Handle admin commands first
		if matches := commandRegex.FindStringSubmatch(strings.TrimSpace(messageText)); matches != nil {
			handleAdminCommand(v, matches[1], matches[2])
			return
		}
		
		// Handle expense messages
		if matches := expenseRegex.FindStringSubmatch(strings.TrimSpace(messageText)); matches != nil {
			processExpenseMessage(v, matches[1], matches[2])
		}
	}
}

func getMessageText(msg *events.Message) string {
	if msg.Message.GetConversation() != "" {
		return msg.Message.GetConversation()
	}
	if msg.Message.GetExtendedTextMessage() != nil {
		return msg.Message.GetExtendedTextMessage().GetText()
	}
	return ""
}

func handleAdminCommand(msg *events.Message, command, args string) {
	var response string
	
	switch command {
	case "help":
		response = `🤖 *WhatsApp Expense Tracker Commands*

📝 *Record Expense:*
Format: \`item amount\`
Example: \`ayam bakar 50000\`

🔧 *Admin Commands:*
• \`/summary\` - Monthly expense summary
• \`/categories\` - List all categories  
• \`/status\` - Bot status
• \`/help\` - Show this help

💡 *Tips:*
• Use clear item names for better categorization
• Amount should be numbers only (50000, not 50k)
• Bot works in group chats only`

	case "status":
		response = fmt.Sprintf(`🤖 *Bot Status*

✅ Status: Online
🕒 Time: %s
📡 Webhook: %s
🤖 LLM: %s
📊 Ready to track expenses!

*Send messages like:* \`nasi gudeg 25000\` 💰`, 
			time.Now().Format("15:04 MST"),
			maskURL(n8nWebhookURL),
			llmProvider)

	case "categories":
		response = `📋 *Available Categories*

🍽️ Food - meals, snacks, groceries
🚗 Transportation - taxi, gas, parking
🛍️ Shopping - clothes, electronics, misc
🎮 Entertainment - movies, games, fun
💡 Bills - utilities, subscriptions
🏥 Health - medicine, doctor visits
📚 Education - books, courses, school
📦 Other - miscellaneous expenses

*Categories are auto-assigned by AI* 🤖`

	case "summary":
		response = `📊 *Monthly Summary*

Coming soon! This will show:
• 💰 Total expenses this month
• 📈 Top categories
• 👥 Group member contributions
• 📅 Daily averages

For now, check your Google Sheet directly! 📋`

	default:
		response = "❓ Unknown command. Type `/help` for available commands."
	}
	
	sendMessage(msg.Info.Chat, response)
}

func processExpenseMessage(msg *events.Message, item string, amountStr string) {
	// Clean amount string - handle both comma and dot separators
	amountStr = strings.ReplaceAll(amountStr, ",", "")
	amountStr = strings.ReplaceAll(amountStr, ".", "")
	
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		log.Printf("❌ Failed to parse amount: %s", amountStr)
		sendMessage(msg.Info.Chat, "❌ Invalid amount format. Use numbers only (e.g., 50000)")
		return
	}
	
	// Get group info
	groupInfo, err := getGroupInfo(msg.Info.Chat)
	if err != nil {
		log.Printf("❌ Failed to get group info: %v", err)
		groupInfo = &GroupInfo{Name: "Unknown Group"}
	}
	
	// Quick categorization first, then LLM if needed
	category := quickCategorize(item)
	if category == "Uncategorized" {
		category = categorizeWithLLM(item)
	}
	
	expenseData := ExpenseData{
		Item:        strings.TrimSpace(item),
		Amount:      amount,
		Category:    category,
		GroupName:   groupInfo.Name,
		SenderName:  msg.Info.PushName,
		SenderPhone: msg.Info.Sender.User,
		Timestamp:   msg.Info.Timestamp,
		MessageID:   msg.Info.ID,
	}
	
	// Send to n8n webhook
	if err := sendToN8N(expenseData); err != nil {
		log.Printf("❌ Failed to send to n8n: %v", err)
		sendMessage(msg.Info.Chat, "❌ Failed to record expense. Please try again.")
		return
	}
	
	// Success confirmation
	confirmationMsg := fmt.Sprintf(`✅ *Expense Recorded*

📝 Item: %s
💰 Amount: Rp %s
🏷️ Category: %s
👤 By: %s

_Saved to %s expenses_ 📊`, 
		expenseData.Item,
		formatCurrency(expenseData.Amount),
		expenseData.Category,
		expenseData.SenderName,
		expenseData.GroupName)
	
	sendMessage(msg.Info.Chat, confirmationMsg)
	
	log.Printf("✅ Expense processed: %s | %s | %.0f | %s", 
		expenseData.GroupName, expenseData.Item, expenseData.Amount, expenseData.Category)
}

func quickCategorize(item string) string {
	itemLower := strings.ToLower(item)
	
	for keyword, category := range categoryKeywords {
		if strings.Contains(itemLower, keyword) {
			return category
		}
	}
	
	return "Uncategorized"
}

func categorizeWithLLM(item string) string {
	if llmProvider == "" || llmAPIKey == "" {
		return "Uncategorized"
	}
	
	prompt := fmt.Sprintf(`Categorize this Indonesian expense item into exactly one category: Food, Transportation, Shopping, Entertainment, Bills, Health, Education, Other.

Item: %s

Reply with only the category name.`, item)
	
	var apiURL string
	var requestBody interface{}
	
	switch llmProvider {
	case "deepseek":
		apiURL = "https://api.deepseek.com/v1/chat/completions"
		requestBody = map[string]interface{}{
			"model": "deepseek-chat",
			"messages": []map[string]string{
				{"role": "user", "content": prompt},
			},
			"max_tokens": 10,
			"temperature": 0.1,
		}
	case "openai":
		apiURL = "https://api.openai.com/v1/chat/completions"
		requestBody = map[string]interface{}{
			"model": "gpt-3.5-turbo",
			"messages": []map[string]string{
				{"role": "user", "content": prompt},
			},
			"max_tokens": 10,
			"temperature": 0.1,
		}
	default:
		return "Other"
	}
	
	jsonData, _ := json.Marshal(requestBody)
	httpClient := &http.Client{Timeout: 15 * time.Second}
	
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("❌ LLM request creation failed: %v", err)
		return "Other"
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+llmAPIKey)
	
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("❌ LLM API call failed: %v", err)
		return "Other"
	}
	defer resp.Body.Close()
	
	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&llmResp); err != nil {
		log.Printf("❌ LLM response decode failed: %v", err)
		return "Other"
	}
	
	if len(llmResp.Choices) > 0 {
		category := strings.TrimSpace(llmResp.Choices[0].Message.Content)
		if category != "" {
			return category
		}
	}
	
	return "Other"
}

func sendToN8N(data ExpenseData) error {
	if n8nWebhookURL == "" {
		return fmt.Errorf("N8N webhook URL not configured")
	}
	
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %v", err)
	}
	
	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Post(n8nWebhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("webhook returned status: %d", resp.StatusCode)
	}
	
	log.Printf("✅ Data sent to N8N successfully")
	return nil
}

func sendMessage(chatID types.JID, message string) {
	if client == nil {
		log.Printf("❌ WhatsApp client not initialized")
		return
	}

	msg := &waE2E.Message{
		Conversation: proto.String(message),
	}

	_, err := client.SendMessage(context.Background(), chatID, msg)
	if err != nil {
		log.Printf("❌ Failed to send message: %v", err)
	} else {
		log.Printf("✅ Message sent to %s", chatID)
	}
}

func formatCurrency(amount float64) string {
	return fmt.Sprintf("%,.0f", amount)
}

func maskURL(url string) string {
	if len(url) > 50 {
		return url[:30] + "..." + url[len(url)-10:]
	}
	return url
}

type GroupInfo struct {
	Name string
}

func getGroupInfo(chatID types.JID) (*GroupInfo, error) {
	if client == nil {
		return nil, fmt.Errorf("client not initialized")
	}

	groupInfo, err := client.GetGroupInfo(chatID)
	if err != nil {
		return nil, err
	}

	return &GroupInfo{Name: groupInfo.Name}, nil
}