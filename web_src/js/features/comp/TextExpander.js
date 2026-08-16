import {matchEmoji, matchMention, matchIssue} from '../../utils/match.js';
import {emojiHTML, emojiString} from '../emoji.js';
import {getIssueIcon, getIssueColor} from '../issue.js';
import {svg} from '../../svg.js';
import {createElementFromHTML} from '../../utils/dom.js';
import {parseIssueHref, parseRepoOwnerPathInfo} from '../../utils.js';
import {debounce} from 'throttle-debounce';

const {customEmojis} = window.config;

// throttle-debounce does not return values, so we need to wrap it
// see https://github.com/niksy/throttle-debounce/issues/59#issuecomment-1185634656
const debouncePromise = (delay, fn) => {
  let resolveCallback;
  const debouncedFn = debounce(delay, async (...args) => {
    const result = await fn(...args);
    if (resolveCallback) {
      resolveCallback(result);
    }
  });
  return (...args) => {
    return new Promise((resolve) => {
      resolveCallback = resolve;
      debouncedFn(...args);
    });
  };
};

const issueSuggestions = debouncePromise(300, async (text, key) => {
  const issuePathInfo = parseIssueHref(window.location.href);
  if (!issuePathInfo.owner) {
    const repoOwnerPathInfo = parseRepoOwnerPathInfo(window.location.pathname);
    issuePathInfo.owner = repoOwnerPathInfo.owner;
    issuePathInfo.repo = repoOwnerPathInfo.repo;
    // then no issuePathInfo.indexString here, it is only used to exclude the current issue when "matchIssue"
  }

  let isPull;
  if (key === '!') {
    isPull = true;
  }

  const matches = await matchIssue(issuePathInfo.owner, issuePathInfo.repo, issuePathInfo.index, text, isPull);
  if (!matches.length) return {matched: false};

  const ul = document.createElement('ul');
  ul.classList.add('suggestions');
  for (const issue of matches) {
    const li = document.createElement('li');
    li.setAttribute('role', 'option');
    li.setAttribute('data-value', `${issue.is_pr ? '!' : '#'}${issue.number}`);
    li.classList.add('tw-flex', 'tw-gap-2');

    const icon = svg(getIssueIcon(issue), 16, ['text', getIssueColor(issue)].join(' '));
    li.append(createElementFromHTML(icon));

    const id = document.createElement('span');
    id.textContent = issue.number.toString();
    li.append(id);

    const nameSpan = document.createElement('span');
    nameSpan.textContent = issue.title;
    li.append(nameSpan);

    ul.append(li);
  }

  return {matched: true, fragment: ul};
});

export function initTextExpander(expander) {
  if (!expander) return;

  expander.addEventListener('text-expander-change', ({detail: {key, provide, text}}) => {
    if (key === ':') {
      const matches = matchEmoji(text);
      if (!matches.length) return provide({matched: false});

      const ul = document.createElement('ul');
      ul.classList.add('suggestions');
      for (const name of matches) {
        const li = document.createElement('li');
        li.setAttribute('id', `combobox-emoji-${name}`);
        li.setAttribute('role', 'option');
        li.setAttribute('data-value', emojiString(name));
        if (customEmojis.has(name)) {
          li.style.gap = '0.25rem';
          li.innerHTML = emojiHTML(name);
          li.append(name);
        } else {
          li.textContent = `${emojiString(name)} ${name}`;
        }
        ul.append(li);
      }

      provide({matched: true, fragment: ul});
    } else if (key === '@') {
      const matches = matchMention(text);
      if (!matches.length) return provide({matched: false});

      const ul = document.createElement('ul');
      ul.classList.add('suggestions');
      for (const {value, name, fullname, avatar} of matches) {
        const li = document.createElement('li');
        li.setAttribute('id', `combobox-user-${name}`);
        li.setAttribute('role', 'option');
        li.setAttribute('data-value', `${key}${value}`);

        const img = document.createElement('img');
        img.setAttribute('aria-hidden', 'true');
        img.src = avatar;
        li.append(img);

        const nameSpan = document.createElement('span');
        nameSpan.textContent = name;
        li.append(nameSpan);

        if (fullname && fullname.toLowerCase() !== name) {
          const fullnameSpan = document.createElement('span');
          fullnameSpan.classList.add('fullname');
          fullnameSpan.textContent = fullname;
          li.append(fullnameSpan);
        }

        ul.append(li);
      }

      provide({matched: true, fragment: ul});
    } else if (key === '#' || key === '!') {
      provide(issueSuggestions(text, key));
    }
  });
  expander.addEventListener('text-expander-value', ({detail}) => {
    if (detail?.item) {
      // add a space after @mentions as it's likely the user wants one
      const suffix = detail.key === '@' ? ' ' : '';
      detail.value = `${detail.item.getAttribute('data-value')}${suffix}`;
    }
  });
}
