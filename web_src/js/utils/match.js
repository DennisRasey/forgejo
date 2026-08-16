import {emojiKeys} from '../features/emoji.js';
import {GET} from '../modules/fetch.js';

const maxMatches = 6;

function sortAndReduce(map) {
  const sortedMap = new Map(Array.from(map.entries()).sort((a, b) => a[1] - b[1]));
  return Array.from(sortedMap.keys()).slice(0, maxMatches);
}

export function matchEmoji(queryText) {
  const query = queryText.toLowerCase().replaceAll('_', ' ');
  if (!query) return emojiKeys.slice(0, maxMatches);

  // results is a map of weights, lower is better
  const results = new Map();
  for (const emojiKey of emojiKeys) {
    const index = emojiKey.replaceAll('_', ' ').indexOf(query);
    if (index === -1) continue;
    results.set(emojiKey, index);
  }

  return sortAndReduce(results);
}

export function matchMention(queryText) {
  const query = queryText.toLowerCase();

  // results is a map of weights, lower is better
  const results = new Map();
  for (const obj of window.config.mentionValues ?? []) {
    const index = obj.key.toLowerCase().indexOf(query);
    if (index === -1) continue;
    const existing = results.get(obj);
    results.set(obj, existing ? existing - index : index);
  }

  return sortAndReduce(results);
}

export async function matchIssue(owner, repo, issueIndexStr, query, isPull) {
  let url = `${window.config.appSubUrl}/${owner}/${repo}/issues/suggestions?q=${encodeURIComponent(query)}`;
  if (isPull !== undefined) {
    url += `&pull=${isPull ? '1' : '0'}`;
  }
  const res = await GET(url);

  const issues = await res.json();
  const issueIndex = parseInt(issueIndexStr);

  // filter out issue with same number
  return issues.filter((i) => i.number !== issueIndex);
}
