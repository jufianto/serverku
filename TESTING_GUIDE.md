# Testing and Setup Guide: DigitalOcean & Notifications

This guide will help you configure and test the newly added **DigitalOcean Provider** and **Notifications** features for `serverku`.

## Prerequisites

Before testing, you need to compile the latest version of the CLI with your changes:

```bash
# Build the binary
go build -o serverku ./cmd/serverku

# Move it to your bin path (optional, or just run ./serverku)
# mv serverku /usr/local/bin/
```

## 1. Testing the DigitalOcean Provider

### Step 1: Obtain a DigitalOcean API Token
1. Log into your [DigitalOcean Control Panel](https://cloud.digitalocean.com/).
2. Click on **API** in the left-hand menu.
3. Click **Generate New Token**. Give it read/write permissions.
4. Copy the token. **You will only see it once.**

### Step 2: Set the Environment Variable
Export the token in your terminal session before running `serverku`:

```bash
export DIGITALOCEAN_TOKEN="your_personal_access_token_here"
```

### Step 3: Create a Test Project Config
Initialize a new project, or manually create a YAML file at `~/.serverku/projects/do-test.yaml`:

```yaml
name: do-test
provider: digitalocean
region: nyc1
vm:
  size: s-1vcpu-1gb
  image: ubuntu-22-04-x64
  spot: false
storage:
  enabled: true
  size_gb: 10
  mount_path: /data
```

### Step 4: Run the Lifecycle
Test the creation, status, and teardown:

```bash
# Spin up the Droplet and Volume
./serverku up do-test

# SSH into it using the provided IP
ssh serverku@<external_ip>

# Tear it down (preserves the volume)
./serverku down do-test

# Permanently destroy everything (Droplet + Volume)
./serverku destroy do-test
```

---

## 2. Testing Notifications (Slack & Telegram)

### Step 1: Setup Slack Webhook (Optional)
1. Go to your Slack workspace and create a new **App**.
2. Enable **Incoming Webhooks**.
3. Create a new webhook for a specific channel.
4. Copy the Webhook URL (e.g., `https://hooks.slack.com/services/T00...`).

### Step 2: Setup Telegram Bot (Optional)
1. Open Telegram and search for the `@BotFather`.
2. Send `/newbot` and follow the prompts to get your **Bot Token** (e.g., `123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11`).
3. Send a message to your new bot.
4. Visit `https://api.telegram.org/bot<YourBOTToken>/getUpdates` in your browser to find your **Chat ID** (look for `"chat":{"id":123456789}`).

### Step 3: Update Project Config
Modify your project config file (e.g., `~/.serverku/projects/do-test.yaml`) to include the `notifications` block:

```yaml
name: do-test
provider: digitalocean
region: nyc1
vm:
  size: s-1vcpu-1gb
# ... other config ...
notifications:
  slack:
    webhook_url: "https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
  telegram:
    bot_token: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
    chat_id: "123456789"
```

### Step 4: Trigger Notifications
Run the lifecycle commands. You should receive messages in your Slack channel and/or Telegram chat.

1. **Test `SendUp`:**
   ```bash
   ./serverku up do-test
   ```
   *Expect a message like: `✅ serverku: Project do-test is up and running at <IP>`*

2. **Test `SendError`:**
   Temporarily break your config (e.g., set an invalid `size: invalid-size`) and try to bring it up.
   ```bash
   ./serverku up do-test
   ```
   *Expect an error message sent to your chats.*

3. **Test `SendDown`:**
   ```bash
   ./serverku down do-test
   ```
   *Expect a message like: `🛑 serverku: Project do-test has been shut down`*
