import {renderMermaid} from './mermaid.js';
import {renderMath} from './math.js';
import {renderCodeCopy} from './codecopy.js';
import {renderAsciicast} from './asciicast.js';
import {renderExternal} from './external.js';
import {initMarkupTasklist} from './tasklist.js';
import {renderSuggestions} from './suggestion.js';

// code that runs for all markup content
export function initMarkupContent() {
  renderMermaid();
  renderMath();
  renderCodeCopy();
  renderAsciicast();
  renderExternal();
}

// code that only runs for comments
export function initCommentContent() {
  initMarkupTasklist();
  renderSuggestions();
}
