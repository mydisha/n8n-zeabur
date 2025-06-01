# 🚀 WhatsApp Expense Tracker Setup Guide

## 📋 Prerequisites

1. **Zeabur Account**: Sign up at [zeabur.com](https://zeabur.com)
2. **Google Cloud Account**: For Google Sheets API
3. **LLM API Key**: Choose one:
   - **DeepSeek** (Recommended - cheapest): $0.14/1M tokens
   - **OpenAI GPT-3.5**: $0.50/1M tokens  
   - **Google Gemini**: $0.35/1M tokens
4. **GitHub Repository**: To store your code

## 🔧 Step-by-Step Setup

### 1. Prepare Your Repository

```bash
# Clone or create a new repository
git clone https://github.com/your-username/whatsapp-expense-tracker
cd whatsapp-expense-tracker

# Copy all the provided files:
# - main.go (WhatsApp bot code)
# - go.mod (Go dependencies)
# - Dockerfile (Container configuration)
# - .env (Environment variables template)
```

### 2. Google Sheets API Setup

1. Go to [Google Cloud Console](https://console.cloud.google.com)
2. Create a new project or select existing one
3. Enable **Google Sheets API**
4. Create **Service Account** credentials:
   - Go to IAM & Admin > Service Accounts
   - Create new service account
   - Generate JSON key file
5. Share your future spreadsheets with the service account email

### 3. Deploy n8n on Zeabur

1. **Create New Project** in Zeabur dashboard
2. **Add Service** > **Prebuilt Service** > Search "n8n"
3. **Configure Environment Variables**:
   ```
   N8N_BASIC_AUTH_ACTIVE=true
   N8N_BASIC_AUTH_USER=admin
   N8N_BASIC_AUTH_PASSWORD=your-secure-password
   N8N_HOST=0.0.0.0
   N8N_PORT=5678
   ```
4. **Deploy** and note the generated URL (e.g., `https://n8n-xxx.zeabur.app`)

### 4. Setup n8n Workflow

1. **Access n8n** using the URL and credentials from step 3
2. **Import Workflow**:
   - Go to Workflows
   - Click "Import from File"
   - Copy-paste the provided JSON workflow
3. **Configure Google Sheets Node**:
   - Click on Google Sheets nodes
   - Add Google Sheets credential
   - Upload the JSON key file from step 2
4. **Activate Webhook**:
   - Click on Webhook node
   - Copy the webhook URL (e.g., `https://n8n-xxx.zeabur.app/webhook/whatsapp-expense`)
5. **Save and Activate** the workflow

### 5. Deploy WhatsApp Bot on Zeabur

1. **Add Git Service** in the same Zeabur project:
   - Click "Add Service" > "Git Service"
   - Connect your GitHub repository
   - Select the repository with WhatsApp bot code

2. **Configure Environment Variables**:
   ```
   PORT=8080
   N8N_WEBHOOK_URL=https://your-n8n-instance.zeabur.app/webhook/whatsapp-expense
   LLM_PROVIDER=deepseek
   LLM_API_KEY=your-deepseek-api-key
   WHATSAPP_SESSION_PATH=/tmp/whatsapp-session
   ```

3. **Deploy** and wait for build completion

### 6. WhatsApp Bot Authentication

1. **Access Bot Interface**:
   - Open your bot URL (e.g., `https://whatsapp-bot-xxx.zeabur.app`)
   - Navigate to `/app/login` endpoint

2. **Scan QR Code**:
   - Use WhatsApp > Linked Devices > Link a Device
   - Scan the QR code displayed
   - Wait for "Connected" status

3. **Test Connection**:
   - Send a test message to yourself
   - Check bot logs in Zeabur dashboard

### 7. Test the Complete Flow

1. **Add Bot to WhatsApp Groups**:
   - Add the phone number (linked to bot) to your expense groups

2. **Test Expense Recording**:
   ```
   # In WhatsApp group, send:
   ayam bakar 50000
   nasi gudeg 25000
   grab transport 15000
   ```

3. **Verify in Google Sheets**:
   - Check if separate spreadsheets are created for each group
   - Verify data accuracy and categorization

## 💡 LLM Cost Optimization Tips

### DeepSeek (Recommended - Cheapest)
- **Cost**: ~$0.14 per 1M tokens
- **API**: `https://api.deepseek.com/v1/chat/completions`
- **Model**: `deepseek-chat`
- **Monthly Cost**: ~$2-5 for moderate usage

### Usage Optimization:
1. **Batch Processing**: Group multiple items in one API call
2. **Caching**: Store common categorizations
3. **Fallback**: Use simple keyword matching for common items
4. **Rate Limiting**: Limit API calls per minute

## 🔒 Security Best Practices

1. **Environment Variables**: Never commit API keys to Git
2. **Webhook Security**: Add API key validation to n8n webhook
3. **Access Control**: Restrict bot to specific groups/users
4. **Data Validation**: Sanitize all inputs
5. **Error Handling**: Don't expose sensitive info in error messages

## 🐛 Troubleshooting

### Common Issues:

**Bot Not Responding:**
- Check WhatsApp connection status
- Verify webhook URL is accessible
- Check Zeabur logs for errors

**Spreadsheet Not Created:**
- Verify Google Sheets API credentials
- Check service account permissions
- Test n8n workflow manually

**LLM Categorization Failing:**
- Verify API key and provider settings
- Check API rate limits
- Test API endpoint directly

**Duplicate Expenses:**
- Check message ID generation logic
- Verify duplicate detection in n8n
- Clear cache if needed

### Debug Commands:

```bash
# Check bot logs
zeabur logs whatsapp-bot

# Test webhook manually
curl -X POST https://your-n8n.zeabur.app/webhook/whatsapp-expense \
  -H "Content-Type: application/json" \
  -d '{"item":"test item","amount":10000,"group_name":"test group","sender_name":"test user","timestamp":"2025-06-01T00:00:00Z"}'
```

## 🚀 Advanced Features (Optional)

### 1. Admin Commands
Add these patterns to your bot:
- `/summary` - Get monthly group expenses
- `/categories` - List all expense categories
- `/export` - Generate expense report

### 2. Multi-Currency Support
- Detect currency from message
- Convert to base currency (IDR)
- Store original and converted amounts

### 3. Receipt Processing
- OCR integration for receipt images
- Automatic item and amount extraction
- Image storage in cloud

### 4. Budget Alerts
- Set monthly budget limits per group
- Send alerts when approaching limits
- Weekly/monthly expense summaries

### 5. Analytics Dashboard
- Create n8n workflow for data visualization
- Connect to Google Data Studio
- Generate automated reports

## 📊 Expected Costs

### Monthly Operational Costs:
- **Zeabur Hosting**: $5-15/month (depending on usage)
- **LLM API (DeepSeek)**: $2-5/month (for moderate usage)
- **Google Sheets API**: Free (under quotas)
- **Total**: ~$7-20/month

### Cost per Transaction:
- **DeepSeek Categorization**: ~$0.0001 per expense
- **Very affordable** for personal/small group use

## 🎉 You're All Set!

Your WhatsApp expense tracker is now ready! Here's what it can do:

✅ **Automatic expense tracking** from WhatsApp groups  
✅ **Smart categorization** using AI  
✅ **Separate spreadsheets** for each group  
✅ **Real-time data sync** to Google Sheets  
✅ **Duplicate prevention** and data validation  
✅ **Scalable architecture** on Zeabur  

**Next Steps:**
1. Test with different expense formats
2. Add more WhatsApp groups
3. Customize categories for your needs
4. Set up regular backups
5. Monitor usage and costs

Happy expense tracking! 💰📊