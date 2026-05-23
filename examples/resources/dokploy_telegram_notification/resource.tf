resource "dokploy_telegram_notification" "alerts" {
  name      = "telegram-ops"
  bot_token = var.telegram_bot_token
  chat_id   = var.telegram_chat_id
}
