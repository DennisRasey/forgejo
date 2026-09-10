import {renderCodeCopy} from './codecopy.js';

test('renderCodeCopy adds the button once and only updates the clipboard text afterwards', () => {
  document.body.innerHTML = '<div class="markup"><pre class="code-block"><code>initial content\n</code></pre></div>';
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
