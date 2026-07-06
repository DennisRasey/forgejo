import {AnsiUp} from 'ansi_up';

const replacements = [
  [/\x1b\[\d+[A-H]/g, ''], // Move cursor, treat them as no-op
  [/\x1bM/g, ''], // Move cursor one line up, threat them as no-op.
  [/\x1b\[\d?[JK]/g, '\r'], // Erase display/line, treat them as a Carriage Return
  [/\x1b\]9;\d+.*?(\x07|\x1b\\)/g, ''], // ConEmu, treat them as no-op.
];

// render ANSI to HTML
export function renderAnsi(line) {
  // create a fresh ansi_up instance because otherwise previous renders can influence
  // the output of future renders, because ansi_up is stateful and remembers things like
  // unclosed opening tags for colors.
  const ansi_up = new AnsiUp();
  ansi_up.use_classes = true;

  if (line.endsWith('\r\n')) {
    line = line.substring(0, line.length - 2);
  } else if (line.endsWith('\n')) {
    line = line.substring(0, line.length - 1);
  }

  if (line.includes('\x1b')) {
    for (const [regex, replacement] of replacements) {
      line = line.replace(regex, replacement);
    }
  }

  if (!line.includes('\r')) {
    return ansi_up.ansi_to_html(line);
  }

  // handle "\rReading...1%\rReading...5%\rReading...100%",
  // convert it into a multiple-line string: "Reading...1%\nReading...5%\nReading...100%"
  const lines = [];
  for (const part of line.split('\r')) {
    if (part === '') continue;
    const partHtml = ansi_up.ansi_to_html(part);
    if (partHtml !== '') {
      lines.push(partHtml);
    }
  }

  // the log message element is with "white-space: break-spaces;", so use "\n" to break lines
  return lines.join('\n');
}

// link schemes ansi_up is allowed to emit; re-checked below because log output is
// untrusted and a javascript: or data: href should not become clickable.
const allowedLinkSchemes = new Set(['http:', 'https:']);

// ansi_up renders OSC 8 hyperlink escape codes as <a> tags with the href and text already
// escaped, but without link attributes. add them here, and drop any link whose scheme is
// not http(s) by replacing it with its text. the html is parsed in a detached <template>,
// which runs no scripts and loads no resources, and the href and text are re-escaped when
// it is serialized back to a string.
function hardenRenderedAnsiLinks(html) {
  // the usual log line has no link, so skip the parse when there is no <a> to touch
  if (!html.includes('<a ')) return html;

  const template = document.createElement('template');
  template.innerHTML = html;

  for (const link of template.content.querySelectorAll('a')) {
    let scheme = '';
    try {
      scheme = new URL(link.getAttribute('href') ?? '').protocol;
    } catch {
      // an empty, relative or malformed href has no scheme and is dropped below
    }
    if (!allowedLinkSchemes.has(scheme)) {
      link.replaceWith(document.createTextNode(link.textContent));
      continue;
    }
    link.setAttribute('target', '_blank');
    link.setAttribute('rel', 'noopener noreferrer nofollow');
  }

  return template.innerHTML;
}

// render ANSI to HTML, turning OSC 8 hyperlink escape codes into links. a job opts in by
// emitting OSC 8, the escape code terminals use for links, so output from a standalone
// Forgejo Runner is linked too. plain-text URLs are left as text.
export function renderAnsiWithLinks(line) {
  return hardenRenderedAnsiLinks(renderAnsi(line));
}
