---
name: local-models
description: Choose which local AI model on the drive to run on this computer, by its memory and the context length a task needs, set it as the drive's default, and tell the user how to start it. Uses only the models already on the drive.
requires: [llama-server]
---

## Steps

`<folder>` is the skill folder that `skill_read` returned, for example
`/Volumes/SVALBARD/skills/skills-core/local-models`.

1. See what the drive has and what fits:

   ```sh
   sh <folder>/scripts/models.sh
   ```

   It prints this computer's memory, each chat model with its size and the
   largest context that fits (Fast 8K, Balanced 32K, Max 128K tokens), the
   current settings, and a recommendation.
2. Pick with the user's task in mind:
   - quick questions, an 8 GB computer: a small model at Fast;
   - an AI client that edits files or reads long documents: Balanced or
     more (opencode needs Balanced);
   - better answers matter more than speed: the largest model that fits at
     the context needed.
   A model marked "does not fit" would swap or crash; do not suggest it.
3. If the user agrees, save the choice on the drive (it applies to the next
   start, not to the model you are running on now):

   ```sh
   "$DRIVE_ROOT/run" settings ai.default_model Qwen_Qwen3.5-4B-Q4_K_M
   "$DRIVE_ROOT/run" settings ai.context balanced
   ```

   The model name is the file name without `.gguf`. `--reset <key>` undoes it.
4. Tell the user how to start it: open a terminal in the drive's folder,
   run `./run`, choose Local AI, then Chat or an AI client.
5. Check your result: write the chosen file name to `choice.txt` and run
   (it must print `ok`):

   ```sh
   sh <folder>/examples/check.sh choice.txt
   ```

## Rules

- Only the models in the drive's `models/` folder exist here. Never
  download one. When none fits, say so; a model can be added later with
  `svalbard add` on a computer with a network.
- Memory rule of thumb: model file + context memory (about 8% of the file
  per 8K tokens, at least 0.5 GB) + 1 GB must stay under 75% of RAM.
- Bigger context is slower: a model answers sooner at Fast than at Max.
