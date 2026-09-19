import emojis from '../../../assets/emoji.json';

const {assetUrlPrefix, customEmojis} = window.config;

const tempMap = Object.assign(...Array.from(customEmojis, (v) => ({[v]: `:${v}:`})));
for (const [emoji, aliases] of emojis) {
  for (const alias of aliases) {
    tempMap[alias] = emoji;
  }
}

export const emojiKeys = Object.keys(tempMap).sort((a, b) => {
  if (b === '+1' && a === '-1') return 1;
  if (a === '+1' || a === '-1') return -1;
  if (b === '+1' || b === '-1') return 1;
  return a.localeCompare(b);
});

const emojiMap = {};
for (const key of emojiKeys) {
  emojiMap[key] = tempMap[key];
}

// retrieve HTML for given emoji name
export function emojiHTML(name) {
  let inner;
  if (customEmojis.has(name)) {
    inner = `<img alt=":${name}:" src="${assetUrlPrefix}/img/emoji/${name}.png">`;
  } else {
    inner = emojiString(name);
  }

  return `<span class="emoji" title=":${name}:">${inner}</span>`;
}

// retrieve string for given emoji name
export function emojiString(name) {
  return emojiMap[name] || `:${name}:`;
}
