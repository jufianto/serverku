# Tutorial 4 — Notifications: never pay for a forgotten VM

*Wire up push notifications in minutes — ntfy with zero accounts, Telegram
with an auto-configuring wizard — and let the VM itself remind you it's
still billing you.*

serverku exists to save budget, and the most expensive VM is the one you
forgot about on Friday evening. This tutorial sets up the notification
stack that makes that impossible:

1. **Lifecycle notifications** — your phone buzzes on every `up`, `down`,
   and error.
2. **The still-running heartbeat** — while a VM runs, it pings you every
   few hours with its uptime and what it has cost so far, *even when your
   laptop is off*.

Two channels are supported. Pick one (or both):

| | ntfy (recommended) | Telegram |
| --- | --- | --- |
| Account needed | none | a bot via @BotFather |
| Setup time | ~3 minutes | ~5 minutes |
| Secret placed on the VM for heartbeats | a random topic name (tied to nothing) | your bot token (root-only, revocable) |
| Worst case if the VM is compromised | spam to one topic | spam as your bot + read messages sent to the bot |

## Part 1 — ntfy in three minutes

[ntfy](https://ntfy.sh) is account-less push: publishing is a plain HTTP
POST to a topic, and **the topic name is the only credential**. serverku
already generated an unguessable topic for your project at `init`
(`serverku-<project>-<12 random hex>`).

### 1. See your topic

```bash
serverku ntfy myapp
```

```text
Push notifications for project "myapp"

  Server: https://ntfy.sh
  Topic:  serverku-myapp-8d17e4e46d6c
  ...
```

Treat the topic like a password — anyone who knows it can send you
notifications (and only that).

### 2. Subscribe on your phone

- **iPhone**: App Store → **ntfy** (free) → **+** → *Subscribe to topic* →
  paste the topic
- **Android**: Play Store or F-Droid → **ntfy** → **+** → paste the topic
- **No phone handy?** Open `https://ntfy.sh/<topic>` in any browser — it's
  a live feed.

iPhone note: out of the box this works with the public ntfy.sh server
(free, donation-supported). Self-hosting the server also works, but iOS
push then relays a wake-up signal through ntfy.sh — an Apple/APNs
constraint, not an ntfy one.

### 3. Prove the pipe

```bash
serverku ntfy myapp --test
```

Your phone should buzz within a second or two:

> **serverku: test notification**
> Subscription for project myapp works. You will receive lifecycle and
> heartbeat notifications here.

That's it. From now on, every `serverku up`/`down`/error for this project
notifies you.

> Prefer being guided? `serverku notify setup myapp` walks through these
> exact steps interactively and won't save anything until you confirm the
> test arrived.

## Part 2 — Telegram with the auto-configuring wizard

Telegram needs a bot, and historically that meant hand-rolling `curl`
calls to find your `chat_id`. serverku's wizard automates everything
Telegram allows to be automated:

### 1. Create a bot (the one manual minute)

Telegram only allows bot creation through a chat with
[@BotFather](https://t.me/BotFather) — there is no API for it, so no tool
can do this for you:

```text
You:       /newbot
BotFather: Alright, a new bot. How are we going to call it?
You:       myapp alerts
BotFather: Good. Now let's choose a username for your bot.
You:       myapp_alerts_bot
BotFather: Done! ... Use this token to access the HTTP API:
           7213456789:AAHfV9xkc3...   ← copy this
```

### 2. Let the wizard do the rest

```bash
serverku notify setup myapp
```

Choose **Telegram**, paste the token, and watch:

```text
Verifying token with Telegram...
Token OK — bot is @myapp_alerts_bot

Step 2 — Open https://t.me/myapp_alerts_bot and send the bot any message.
Waiting for your message (up to 2m0s)...
Found chat with Jufi (chat_id 123456789)

Step 3 — Sending a test notification from this machine...
? Did the bot's test message arrive in Telegram? Yes
```

What just happened, in order:

1. **`getMe`** verified the token is real and showed which bot it belongs
   to — catching typos before anything else.
2. **`getUpdates` polling** discovered your `chat_id` the moment you
   messaged the bot. (Bots cannot message you first — that's why you send
   the first message.)
3. A **real test message** proved end-to-end delivery, and only then was
   the config saved.

### 3. Re-test anytime

```bash
serverku notify test myapp
```

```text
  ✓ ntfy     test notification sent
  ✓ telegram test notification sent

All channels OK.
```

Every configured channel gets a test, with per-channel pass/fail. It exits
non-zero on failure — see the hook trick at the end.

## Part 3 — the heartbeat: the VM that won't let you forget it

Lifecycle notifications only fire when *you* run a command. The heartbeat
covers the dangerous case: the VM you brought up and walked away from.

Enable it in the wizard when asked, or in the config:

```yaml
notifications:
  ntfy:
    topic: serverku-myapp-8d17e4e46d6c
    heartbeat_hours: 6
```

On the next `serverku up`, provisioning installs a small script and a
systemd timer **on the VM itself**. Every 6 hours, for as long as the VM
exists:

> **serverku: myapp still running**
> serverku: myapp still running -- up 18h12m, $0.16 so far ($0.0089/hr).
> Stop with: serverku down myapp

Three properties make this design trustworthy:

- **It works while your laptop is off** — the VM sends it, and the VM is
  by definition on. This is exactly when machines get forgotten.
- **It can never false-alarm** — the timer lives on the VM, so
  `down`/`destroy` removes it with the machine.
- **The cost figure is honest** — a live provider price shows plain
  (`$0.0089/hr`); an offline estimate is always marked (`~$0.03/hr est.`).

### What lands on the VM (the security bit)

The heartbeat must place its sending credential on the VM, in a root-only
(0700) script. Choose your channel accordingly:

- **ntfy**: only the random topic name lands there. It is tied to no
  account. Worst case, an attacker who roots your VM can send
  notifications to that one topic. Rotation = generate a new topic.
- **Telegram**: your bot token lands there. Worst case, an attacker can
  send messages as that bot and read messages people send *to* it. Use a
  bot dedicated to serverku, and `/revoke` it in BotFather if a VM is ever
  compromised.

Your cloud credentials (`DIGITALOCEAN_TOKEN`, GCP ADC) and your SSH
private key **never** leave your machine, heartbeat or not.

If both channels have `heartbeat_hours`, the smaller interval wins and
each ping goes to both — independently, so one channel being down doesn't
silence the other.

## Part 4 — a hook trick: refuse to deploy blind

Since `serverku notify test` exits non-zero on failure, you can make `up`
refuse to create billable resources unless notifications provably work:

```yaml
hooks:
  pre_up:
    - serverku notify test myapp
```

`pre_up` hooks gate the operation — if the test send fails, nothing gets
created, and you fix notifications before spending a cent.

## What you learned

- ntfy: subscribe to a random topic, `--test`, done — no accounts, and
  nothing account-linked ever reaches the VM.
- Telegram: one minute with BotFather; the wizard verifies the token,
  auto-discovers your chat, and test-sends before saving.
- The heartbeat turns "I forgot the VM" from a billing surprise into a
  push notification with a dollar amount on it.
- `serverku notify test` + a `pre_up` hook = never deploy with broken
  notifications.

---

Back to the [tutorial index](README.md) — or jump to
[WordPress with HTTPS](tutorial-wordpress-digitalocean.md) to give these
notifications something worth watching.
