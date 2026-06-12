# transcript-extract.jq — render a Claude Code session JSONL as readable chat.
# Visible text only: founder messages, assistant replies, tool-name trail,
# compaction summaries. Thinking, tool outputs, and system-reminders excluded.
# Used by save-transcript.sh and chat-load.sh (jq -rf).

def textblocks:
  (if (.message.content | type) == "string"
   then [.message.content]
   else [.message.content[]? | select(.type == "text") | .text]
   end)
  | map(select(startswith("<system-reminder>") | not))
  | map(select(length > 0));

select(.isSidechain != true)
| if .type == "summary" then
    "## [COMPACTION SUMMARY]\n\n" + (.summary // empty)
  elif .type == "user" then
    (textblocks
     | select(length > 0)
     | "## USER\n\n" + join("\n\n"))
  elif .type == "assistant" then
    ((textblocks | join("\n\n")) as $t
     | ([.message.content[]? | select(.type == "tool_use") | .name] | unique) as $tools
     | if ($t | length) > 0 then
         "## ASSISTANT\n\n"
         + (if ($tools | length) > 0 then "> [tools: " + ($tools | join(", ")) + "]\n\n" else "" end)
         + $t
       elif ($tools | length) > 0 then
         "> [tools: " + ($tools | join(", ")) + "]"
       else empty
       end)
  else empty
  end
| . + "\n"
