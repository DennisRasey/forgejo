import {renderAnsi, renderAnsiWithLinks} from './ansi.js';

test('renderAnsi', () => {
  expect(renderAnsi('abc')).toEqual('abc');
  expect(renderAnsi('abc\n')).toEqual('abc');
  expect(renderAnsi('abc\r\n')).toEqual('abc');
  expect(renderAnsi('\r')).toEqual('');
  expect(renderAnsi('\rx\rabc')).toEqual('x\nabc');
  expect(renderAnsi('\rabc\rx\r')).toEqual('abc\nx');
  expect(renderAnsi('\x1b[30mblack\x1b[37mwhite')).toEqual('<span class="ansi-black-fg">black</span><span class="ansi-white-fg">white</span>'); // unclosed
  expect(renderAnsi('<script>')).toEqual('&lt;script&gt;');
  expect(renderAnsi('\x1b[1A\x1b[2Ktest\x1b[1B\x1b[1A\x1b[2K')).toEqual('test');
  expect(renderAnsi('\x1b[1A\x1b[2K\rtest\r\x1b[1B\x1b[1A\x1b[2K')).toEqual('test');
  expect(renderAnsi('\x1b[1A\x1b[2Ktest\x1b[1B\x1b[1A\x1b[2K')).toEqual('test');
  expect(renderAnsi('\x1b[1A\x1b[2K\rtest\r\x1b[1B\x1b[1A\x1b[2K')).toEqual('test');

  // treat "\033[0K" and "\033[0J" (Erase display/line) as "\r", then it will be covered to "\n" finally.
  expect(renderAnsi('a\x1b[Kb\x1b[2Jc')).toEqual('a\nb\nc');
  expect(renderAnsi('\x1b[48;5;88ma\x1b[38;208;48;5;159mb\x1b[m')).toEqual(`<span style="background-color:rgb(135,0,0)">a</span><span style="background-color:rgb(175,255,255)">b</span>`);

  expect(renderAnsi('\x1b]9;4;0\x07')).toEqual('');
  expect(renderAnsi('\x1b]9;4;1;25\x07compiling main.zig')).toEqual('compiling main.zig');
  expect(renderAnsi('\x1b]9;4;1;25\x1b\\compiling main.zig')).toEqual('compiling main.zig');
  expect(renderAnsi('\x1b]9;4;3\x07waiting...')).toEqual('waiting...');
  expect(renderAnsi('\x1b]9;1;500\x07sleeping...')).toEqual('sleeping...');
  expect(renderAnsi('\x1b]9;4;3\x07waiting...\x1b]9;4;3\x07')).toEqual('waiting...');
  expect(renderAnsi('\x1b]9;12\x07')).toEqual('');
  expect(renderAnsi('\x1b]9;4;1;25\x07\x1bMcompiling main.zig')).toEqual('compiling main.zig');
});

function renderLinked(line) {
  const div = document.createElement('div');
  div.innerHTML = renderAnsiWithLinks(line);
  return div;
}

// build an OSC 8 hyperlink: ESC ] 8 ; ; <url> ST <text> ESC ] 8 ; ; ST, where the string
// terminator ST is ESC \ (the default) or BEL.
function osc8(url, text, st = '\x1b\\') {
  return `\x1b]8;;${url}${st}${text}\x1b]8;;${st}`;
}

test('renderAnsiWithLinks: an OSC 8 hyperlink becomes an anchor with safe link attributes, showing the link text not the url', () => {
  const div = renderLinked(`See ${osc8('https://example.com/path?x=1#frag', 'the build')}`);
  const link = div.querySelector('a');
  expect(link.getAttribute('href')).toEqual('https://example.com/path?x=1#frag');
  expect(link.textContent).toEqual('the build');
  expect(link.getAttribute('target')).toEqual('_blank');
  expect(link.getAttribute('rel')).toEqual('noopener noreferrer nofollow');
  expect(div.textContent).toEqual('See the build');
});

test('renderAnsiWithLinks: BEL also terminates the sequence', () => {
  expect(renderLinked(osc8('http://example.com', 'x', '\x07')).querySelector('a').getAttribute('href')).toEqual('http://example.com');
});

test('renderAnsiWithLinks: several hyperlinks on one line', () => {
  const div = renderLinked(`${osc8('https://example.com/a', 'a')} and ${osc8('http://example.test/b', 'b')}`);
  expect(Array.from(div.querySelectorAll('a'), (a) => a.getAttribute('href'))).toEqual(['https://example.com/a', 'http://example.test/b']);
});

test('renderAnsiWithLinks: plain-text URLs are left as text; only OSC 8 hyperlinks are linked', () => {
  const div = renderLinked('See https://example.com/path now');
  expect(div.querySelectorAll('a').length).toEqual(0);
  expect(div.textContent).toEqual('See https://example.com/path now');
});

test('renderAnsiWithLinks: a javascript: or data: scheme is never clickable', () => {
  // eslint-disable-next-line no-script-url -- the point of the test is a javascript: link is dropped
  expect(renderLinked(osc8('javascript:alert(1)', 'click')).querySelectorAll('a').length).toEqual(0);
  expect(renderLinked(osc8('data:text/html,<script>alert(1)</script>', 'click')).querySelectorAll('a').length).toEqual(0);
});

test('renderAnsiWithLinks: HTML in the text or href stays escaped, so a hyperlink cannot inject markup or break out of the attribute', () => {
  const div = renderLinked(osc8('https://example.com/"onmouseover=alert(1)', '<b>x</b>'));
  const link = div.querySelector('a');
  expect(link.getAttribute('onmouseover')).toBeNull();
  expect(link.getAttribute('href')).toEqual('https://example.com/"onmouseover=alert(1)');
  expect(link.querySelector('b')).toBeNull();
  expect(link.textContent).toEqual('<b>x</b>');
});

test('renderAnsiWithLinks: text that is itself a url still yields one anchor pointing at the OSC 8 target, not the text', () => {
  const div = renderLinked(osc8('https://example.com/real', 'https://example.com/shown'));
  expect(div.querySelectorAll('a').length).toEqual(1);
  expect(div.querySelector('a').getAttribute('href')).toEqual('https://example.com/real');
});
