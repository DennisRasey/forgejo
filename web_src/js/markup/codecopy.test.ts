import {expect, test} from 'vitest';
import {renderCodeCopy} from './codecopy.ts';

test('renderCodeCopy adds the button once and only updates the clipboard text afterwards', () => {
  document.body.innerHTML = '<div class="markup"><div><pre class="code-block"><code>initial content\n</code></pre></div></div>';
  const code = document.querySelector('code');

  renderCodeCopy();
  renderCodeCopy();

  let buttons = document.querySelectorAll('button.code-copy');
  expect(buttons.length).toEqual(1);
  expect(buttons[0].getAttribute('data-clipboard-text')).toEqual('initial content');

  code.textContent = 'modified content\n';
  renderCodeCopy();

  buttons = document.querySelectorAll('button.code-copy');
  expect(buttons.length).toEqual(1);
  expect(buttons[0].getAttribute('data-clipboard-text')).toEqual('modified content');
});

test('renderCodeCopy skips mermaid blocks', () => {
  document.body.innerHTML = `<div class="markup">
    <div><pre class="code-block"><code class="language-mermaid">graph LR\nA --> B\n</code></pre></div>
    <div><pre class="code-block"><code class="language-go">fmt.Println("copy me")\n</code></pre></div>
  </div>`;

  renderCodeCopy();

  const buttons = document.querySelectorAll('button.code-copy');
  expect(buttons.length).toEqual(1);
  expect(buttons[0].getAttribute('data-clipboard-text')).toEqual('fmt.Println("copy me")');
  expect(document.querySelector('.language-mermaid').closest('pre').nextElementSibling).toBeNull();
});
