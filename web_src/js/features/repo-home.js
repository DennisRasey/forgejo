import {hideElem, showElem} from '../utils/dom.ts';
import {GET, POST} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.js';
import {createTomSelect} from '../modules/tom-select.ts';

const {appSubUrl} = window.config;

export async function initRepoTopicBar() {
  const mgrBtn = document.getElementById('manage_topic');
  if (!mgrBtn) return;

  const editDiv = document.getElementById('topic_edit');
  const viewDiv = document.getElementById('repo-topics');
  const inputEl = editDiv.querySelector('input[name=topics]');
  let lastErrorToast;
  let tomSelect;

  // Store original topics for cancel functionality
  const getOriginalTopics = () => {
    return Array.from(viewDiv.querySelectorAll('.repo-topic'), (el) => el.textContent.trim());
  };

  mgrBtn.addEventListener('click', async () => {
    hideElem(viewDiv);
    showElem(editDiv);

    // Lazy initialize Tom Select on first click
    if (!tomSelect) {
      const originalTopics = getOriginalTopics();

      tomSelect = await createTomSelect(inputEl, {
        plugins: ['remove_button'],
        persist: false,
        createOnBlur: true,
        loadThrottle: 500,
        valueField: 'topic_name',
        labelField: 'topic_name',
        searchField: 'topic_name',
        items: originalTopics,
        render: {
          no_results: () => null,
        },
        async load(query, callback) {
          if (!query.length) return callback([]);
          try {
            const response = await GET(`${appSubUrl}/explore/topics/search?q=${encodeURIComponent(query)}`);
            const data = await response.json();
            // Filter out already selected topics
            const current = this.getValue();
            const filtered = (data.topics || []).filter((t) => !current.includes(t.topic_name));
            callback(filtered);
          } catch {
            callback([]);
          }
        },
        create(input) {
          return {topic_name: input.toLowerCase().trim()};
        },
      });
    }

    tomSelect.focus();
  });

  document.querySelector('#cancel_topic_edit').addEventListener('click', () => {
    lastErrorToast?.hideToast();
    hideElem(editDiv);
    showElem(viewDiv);

    // Reset to original values
    if (tomSelect) {
      const originalTopics = getOriginalTopics();
      tomSelect.clear(true);
      for (const topic of originalTopics) {
        tomSelect.addItem(topic, true);
      }
    }

    mgrBtn.focus();
  });

  document.getElementById('save_topic').addEventListener('click', async (e) => {
    lastErrorToast?.hideToast();

    // Clear any previous invalid state
    if (tomSelect) {
      for (const item of tomSelect.wrapper.querySelectorAll('.item.invalid')) {
        item.classList.remove('invalid');
      }
    }

    const topics = inputEl.value;

    const data = new FormData();
    data.append('topics', topics);

    const response = await POST(e.target.getAttribute('data-link'), {data});

    if (response.ok) {
      const responseData = await response.json();
      if (responseData.status === 'ok') {
        // Update view with new topics
        for (const el of viewDiv.querySelectorAll('.repo-topic')) {
          el.remove();
        }
        if (topics.length) {
          const topicArray = topics.split(',');
          topicArray.sort();
          for (const topic of topicArray) {
            // it should match the code in repo/home.tmpl
            const link = document.createElement('a');
            link.classList.add('repo-topic', 'ui', 'large', 'label');
            link.href = `${appSubUrl}/explore/repos?q=${encodeURIComponent(topic)}&topic=1`;
            link.textContent = topic;
            mgrBtn.parentNode.insertBefore(link, mgrBtn); // insert all new topics before manage button
          }
        }
        hideElem(editDiv);
        showElem(viewDiv);
      }
    } else if (response.status === 422) {
      // how to test: input topic like " invalid topic " (with spaces), and select it from the list, then "Save"
      const responseData = await response.json();
      lastErrorToast = showErrorToast(responseData.message, {duration: 5000});
      if (responseData.invalidTopics?.length > 0) {
        const {invalidTopics} = responseData;
        // Mark invalid topics in Tom Select
        const items = tomSelect.wrapper.querySelectorAll('.ts-control .item');
        const values = topics.split(',');
        for (const [index, value] of values.entries()) {
          if (invalidTopics.includes(value)) {
            items[index]?.classList.add('invalid');
          }
        }
      }
    }
  });
}
