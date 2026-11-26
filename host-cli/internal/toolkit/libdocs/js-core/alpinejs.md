# Alpine.js

Small reactive behaviour written in HTML attributes: state, click handlers, lists, toggles. Write the page with a bare import and pack it with `svalbard-drive html pack`; never link a CDN.

## Import

```js
import Alpine from "alpinejs";
window.Alpine = Alpine;
Alpine.start();
```

## Example

```html
<!doctype html>
<html><head><meta charset="utf-8"><title>Counter</title></head>
<body>
<div x-data="{ count: 0, items: ['milk', 'bread'] }">
  <button @click="count++">Add one</button> <span x-text="count"></span>
  <ul><template x-for="item in items"><li x-text="item"></li></template></ul>
  <input x-model="newItem" x-data="{ newItem: '' }" @keyup.enter="items.push(newItem); newItem = ''">
</div>
<script type="module">
import Alpine from "alpinejs";
window.Alpine = Alpine;
Alpine.start();
</script>
</body></html>
```

## Notes

- `x-data` holds state, `x-text`/`x-html` show it, `@click` (`x-on:click`) runs code, `x-model` binds an input, `x-show`/`x-if` hide, `x-for` repeats a `<template>`.
- Shared logic: `Alpine.data("name", () => ({ ... }))` before `Alpine.start()`, then `x-data="name"`.

## Reference

The Alpine.js docs at this version are in `alpinejs.reference.md` (`skill_read`, file `alpinejs.reference.md`): every directive, magic property and global.
