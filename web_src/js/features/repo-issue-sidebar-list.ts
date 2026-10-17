import $ from 'jquery';
import {htmlEscape} from 'escape-goat';
import {emojiHTML} from './emoji.ts';

const {appSubUrl} = window.config;

// TODO: move this to dropdown.js when we typescript that module:
declare global {
  interface JQuery {
    dropdown: (...args: unknown[]) => JQuery;
  }
}

export function initRepoIssueSidebarList() {
  const repolink = $('#repolink').val();
  const repoId = $('#repoId').val();
  const crossRepoSearch = $('#crossRepoSearch').val() === 'true';
  const tp = $('#type').val();

  $('#new-dependency-drop-list')
    .dropdown({
      apiSettings: {
        beforeSend(settings) {
          if (!settings.urlData.query.trim()) {
            settings.url = `${appSubUrl}/${repolink}/issues/search?q={query}&type=${tp}&sort=updated`;
          } else if (crossRepoSearch) {
            settings.url = `${appSubUrl}/issues/search?q={query}&priority_repo_id=${repoId}&type=${tp}&sort=relevance`;
          } else {
            settings.url = `${appSubUrl}/${repolink}/issues/search?q={query}&type=${tp}&sort=relevance`;
          }
          return settings;
        },
        onResponse(response: Record<string, {
          id: string,
          number: number,
          title: string,
          repository: {
            full_name: string
          }
        }>) {
          const filteredResponse = {success: true, results: []};
          const currIssueId = $('#new-dependency-drop-list').data('issue-id');
          // Parse the response from the api to work with our dropdown
          for (const [_, issue] of Object.entries(response)) {
            // Don't list current issue in the dependency list.
            if (issue.id === currIssueId) {
              continue;
            }
            filteredResponse.results.push({
              name: `#${issue.number} ${issueTitleHTML(htmlEscape(issue.title))
              }<div class="text small tw-break-anywhere">${htmlEscape(issue.repository.full_name)}</div>`,
              value: issue.id,
            });
          }
          return filteredResponse;
        },
        cache: false,
      },

      fullTextSearch: true,
    });

  $('.menu button.label-exclude-item-btn').each(function () {
    $(this).on('click', function () {
      const label = this.closest('.item').querySelector('a.label-filter-item');

      if (!label) {
        return;
      }

      excludeLabel(label);
    });
  });

  // Increase surface area to include a label in the filters
  for (const labelFilterItem of document.querySelectorAll<HTMLAnchorElement>('.menu a.label-filter-item')) {
    const menuItem = labelFilterItem.closest('.item');
    menuItem.addEventListener('click', (event: MouseEvent) => {
      if (labelFilterItem === event.target || (event.target as HTMLElement).closest('.label-exclude-item-btn')) {
        return;
      }

      labelFilterItem.click();
    });
  }

  $('.menu .ui.dropdown.label-filter').on('keydown', (e) => {
    const selectedItem = document.querySelector('.menu .ui.dropdown.label-filter .menu .item.selected');

    if (!selectedItem) {
      return;
    }

    const selectedItemExcludeButton = selectedItem.querySelector('.label-exclude-item-btn');
    const selectExcludeButton = () => selectedItemExcludeButton?.classList.add('selected');
    const deselectExcludeButton = () => selectedItemExcludeButton?.classList.remove('selected');
    const isExcludeButtonSelected = () => selectedItemExcludeButton?.classList.contains('selected');

    if (e.key === 'Enter') {
      const labelElement = selectedItem.querySelector<HTMLAnchorElement>('a.label-filter-item');

      if (!labelElement) {
        return;
      }

      if (isExcludeButtonSelected()) {
        excludeLabel(labelElement);
      } else {
        labelElement.click();
      }
    }

    // the menu can be navigated with or without the search input being focused
    // therefore we check if the input is currently focused and the caret is
    // at the end to make sure the moving the caret within the input works
    const isOnInput = (e.target as HTMLElement).matches('input');
    const input = e.target as HTMLInputElement;

    if (e.key === 'ArrowRight' && (!isOnInput || isCaretAtEnd(input))) {
      selectExcludeButton();
    }

    if (e.key === 'ArrowLeft') {
      // it will deselect the exclude button before letting the user move along the focused input text
      // so the user has to press once the left key to deselect and then another time to
      // move the caret to the left side
      if (isOnInput && isCaretAtEnd(input) && selectedItemExcludeButton.classList.contains('selected')) {
        e.preventDefault();
      }
      deselectExcludeButton();
    }

    // when a exclude button is selected moving to the prev or next item in the menu
    // is still possible, but the exclude button can remain selected, this makes
    // sure to clear the selection class from the exclude buttons that are not
    // within the currently selected menu item
    if (e.key === 'ArrowUp' || e.key === 'ArrowDown') {
      for (const excludeButtonSelected of document.querySelectorAll('.label-exclude-item-btn.selected')) {
        if (!selectedItem.contains(excludeButtonSelected)) {
          excludeButtonSelected.classList.remove('selected');
        }
      }
    }
  });

  $('.ui.dropdown.label-filter, .ui.dropdown.select-label').dropdown('setting', {'hideDividers': 'empty'}).dropdown('refreshItems');
}

/**
 * Render the issue's title.
 * It converts emojis and code blocks syntax into their respective HTML equivalent.
 */
export function issueTitleHTML(title: string) {
  return title.replaceAll(/:[-+\w]+:/g, (emoji) => emojiHTML(emoji.substring(1, emoji.length - 1)))
    .replaceAll(/`[^`]+`/g, (code) => `<code class="inline-code-block">${code.substring(1, code.length - 1)}</code>`);
}

/**
 * Excludes a label from filters provided by the data-label-id attribute of an element.
 *
 * If the label is included it will be converted to an exclusion, if its already excluded it will get removed, otherwise, if not present at all it will get excluded.
 */
export function excludeLabel(item: Element) {
  const id = item.getAttribute('data-label-id');
  const excludedId = `-${id}`;

  const params = new URLSearchParams(window.location.search);
  const labelIds = new Set((params.get('labels') ?? '').split(',').filter((id) => id.length > 0));

  if (labelIds.has(id)) {
    labelIds.delete(id);
    labelIds.add(excludedId);
  } else if (labelIds.has(excludedId)) {
    labelIds.delete(excludedId);
  } else {
    labelIds.add(excludedId);
  }

  params.set('labels', Array.from(labelIds).join(','));

  window.location.search = params.toString();
}

/**
 * Returns true if the caret is at the end of the input even if it has content
 */
function isCaretAtEnd(inputElement: HTMLInputElement) {
  const value = inputElement.value;
  return (
    inputElement.selectionStart === inputElement.selectionEnd &&
    inputElement.selectionEnd === value.length
  );
}
