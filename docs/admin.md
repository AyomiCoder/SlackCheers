# SlackCheers Admin Guide

## What SlackCheers does

SlackCheers sends clean, automatic celebration posts in Slack for birthdays and work anniversaries.

It helps teams celebrate consistently without manual reminders. Admins control where and when messages are posted.

## Setup flow

1. Install the Slack app in your workspace.
2. Choose your celebration channel.
3. Choose your posting time.
4. Team members add birthday and work start dates.

## How dates are collected

Each person can provide:
- Birthday (day/month, year optional)
- Work start date
- Public celebration preference

People can reply directly to the bot DM with one or both lines:
```text
march 25
january 23, 2024
```

`month day` saves birthday. `month day, year` saves hire date (year required).

People can later update their details by sending another DM in the same format.

## Privacy and visibility

- Public celebration toggle controls if a user is included in channel posts.
- Dates are only used for birthday and anniversary reminders.
- Admin dashboard users can manage people/date records for their workspace.

## Weekend behavior

Celebrations run on the exact calendar day, even on weekends.
