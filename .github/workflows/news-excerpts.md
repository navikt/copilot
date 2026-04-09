---
on:
  schedule: "0 9 * * 1,3,5"
  workflow_dispatch:

engine:
  id: copilot
  model: claude-opus-4.6

permissions:
  contents: read

network:
  allowed:
    - defaults
    - github
    - "github.blog"
    - "openai.com"
    - "anthropic.com"
    - "blog.google"
    - "ai.google.dev"
    - "code.visualstudio.com"
    - "devblogs.microsoft.com"

safe-outputs:
  create-pull-request:
    title-prefix: "[news] "
    labels: [news, automated]
    draft: true
    allowed-files:
      - docs/news/articles/**






















































































































































