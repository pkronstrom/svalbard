package wiki

// HelpPage is the page that tells a model, and the user, what a page can do
// beyond text. The wiki tools' descriptions name it instead of carrying the
// syntax in every prompt.
const HelpPage = "Help/Live pages"

// helpPage is written for SilverBullet 3.0.x. Every example on it was run on
// the pinned release; check them again when the pin moves. The examples are
// live, not in code blocks: a small model copies an example with whatever
// surrounds it, and inside a code block the wiki only shows it.
const helpPage = `# Live pages

A page can hold more than text. The wiki runs anything written as ` + "`${...}`" + ` when the page is shown, and puts the result in its place. The examples on this page are live, so what you see are their results. To use one, copy it as it is written in this page's text: on lines of its own, with no code block and no backticks around it. Inside a code block it is only shown.

## A list of pages

The pages in the folder Projects, as links. Change ` + "`Projects/`" + ` to another folder:

${query[[
  from p = index.pages()
  where p.name:startsWith("Projects/")
  order by p.name
  select templates.pageItem(p)
]]}

The five pages changed last:

${query[[
  from p = index.pages()
  order by p.lastModified desc
  limit 5
  select templates.pageItem(p)
]]}

## Counting, dates, any expression

Pages in the wiki, not counting the wiki's own under Library: ${#query[[from p = index.pages() where not p.name:startsWith("Library/")]]}

Today's date: ${os.date("%Y-%m-%d")}

## Tasks

A line that starts with ` + "`- [ ]`" + ` is a task. The open tasks from every page:

${query[[
  from t = index.tasks()
  where not t.done
  select templates.taskItem(t)
]]}

## Rules of a query

Only ` + "`from`" + ` is required. ` + "`where`, `order by`, `limit` and `select`" + ` take Lua expressions. A page has ` + "`name`, `lastModified`, `created` and `tags`" + `. End a query of pages with ` + "`select templates.pageItem(p)`" + ` for a list of links, and a query of tasks with ` + "`select templates.taskItem(t)`" + `.

## Commands

Lua in a block marked ` + "`space-lua`" + ` defines things for the whole wiki. It runs only on a page whose name starts with ` + "`Library/`" + `, for example ` + "`Library/My commands`" + `, and takes effect when the wiki is opened or reloaded. This adds a command to the command list:

` + "```lua" + `
command.define {
  name = "Say hello",
  run = function()
    editor.flashNotification("Hello from the wiki")
  end
}
` + "```" + `

On the Library page the block starts with ` + "```space-lua" + ` instead of ` + "```lua" + `.
`
