// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

import {describe, expect, test, vi} from 'vitest';
import {mount} from '@vue/test-utils';
import ActionJobStep from './ActionJobStep.vue';

vi.mock('../utils/time.js', () => ({
  formatDatetime: vi.fn((date) => date.toISOString()),
}));

describe('ActionJobStep', () => {
  const defaultProps = {
    stepId: 12321,
    status: 'success',
    runStatus: 'success',
    expanded: false,
    isExpandable: vi.fn(() => true),
    isDone: vi.fn(() => true),
    cursor: null,
    summary: 'Build project',
    duration: '2m 30s',
    timeVisibleTimestamp: false,
    timeVisibleSeconds: false,
  };

  function createWrapper(props = {}) {
    return mount(ActionJobStep, {
      props: {
        ...defaultProps,
        ...props,
      },
    });
  }

  describe('rendering', () => {
    test('renders job step summary correctly', () => {
      const wrapper = createWrapper();

      expect(wrapper.find('.job-step-summary').exists()).toBe(true);
      expect(wrapper.find('.step-summary-msg').text()).toBe('Build project');
      expect(wrapper.find('.step-summary-duration').text()).toBe('2m 30s');
    });

    test('shows loading icon when expanded and cursor is null', () => {
      const wrapper = createWrapper({
        expanded: true,
        cursor: null,
      });
      const icons = wrapper.findAllComponents({name: 'SvgIcon'});
      expect(icons[0].props('name')).toBe('octicon-sync');
    });

    test('shows chevron-down when expanded', () => {
      const wrapper = createWrapper({
        expanded: true,
        cursor: 10,
      });
      const icons = wrapper.findAllComponents({name: 'SvgIcon'});
      expect(icons[0].props('name')).toBe('octicon-chevron-down');
    });

    test('shows chevron-right when not expanded', () => {
      const wrapper = createWrapper({
        expanded: false,
      });
      const icons = wrapper.findAllComponents({name: 'SvgIcon'});
      expect(icons[0].props('name')).toBe('octicon-chevron-right');
    });

    test('adds step-expandable class when step is expandable', () => {
      const wrapper = createWrapper();
      expect(wrapper.find('.job-step-summary').classes()).toContain('step-expandable');
    });

    test('does not add step-expandable class when step is not expandable', () => {
      const wrapper = createWrapper({
        isExpandable: vi.fn(() => false),
      });
      expect(wrapper.find('.job-step-summary').classes()).not.toContain('step-expandable');
    });

    test('adds selected class when expanded', () => {
      const wrapper = createWrapper({
        expanded: true,
      });
      expect(wrapper.find('.job-step-summary').classes()).toContain('selected');
    });

    test('hides logs container when not expanded', async () => {
      const wrapper = createWrapper({
        expanded: false,
      });
      const logsContainer = wrapper.find('.job-step-logs');
      // expect(logsContainer.isVisible()).toBe(false); // isVisible doesn't work, even attempting workarounds https://github.com/vuejs/vue-test-utils/issues/2073
      expect(logsContainer.element.style.display).toBe('none');
    });

    test('shows logs container when expanded', () => {
      const wrapper = createWrapper({
        expanded: true,
      });
      const logsContainer = wrapper.find('.job-step-logs');
      expect(logsContainer.isVisible()).toBe(true);
      expect(logsContainer.element.style.display).not.toBe('none'); // since we can't rely on isVisible (see !expanded test)
    });
  });

  describe('events', () => {
    test('emits toggle event on click when expandable', async () => {
      const wrapper = createWrapper();
      await wrapper.find('.job-step-summary').trigger('click');
      expect(wrapper.emitted('toggle')).toBeTruthy();
      expect(wrapper.emitted('toggle')).toHaveLength(1);
    });

    test('does not emit toggle event on click when not expandable', async () => {
      const wrapper = createWrapper({
        isExpandable: vi.fn(() => false),
      });
      await wrapper.find('.job-step-summary').trigger('click');
      expect(wrapper.emitted('toggle')).toBeFalsy();
    });

    test('emits toggle event on Enter key when expandable', async () => {
      const wrapper = createWrapper();
      await wrapper.find('.job-step-summary').trigger('keyup.enter');
      expect(wrapper.emitted('toggle')).toBeTruthy();
    });

    test('emits toggle event on Space key when expandable', async () => {
      const wrapper = createWrapper();
      await wrapper.find('.job-step-summary').trigger('keyup.space');
      expect(wrapper.emitted('toggle')).toBeTruthy();
    });
  });

  describe('appendLogs method', () => {
    test('creates log lines and appends them to container', () => {
      const wrapper = createWrapper();
      const logLines = [
        {index: 1, timestamp: 1765163618, message: 'Starting build'},
        {index: 2, timestamp: 1765163619, message: 'Running tests'},
        {index: 3, timestamp: 1765163620, message: 'Build complete'},
      ];

      wrapper.vm.appendLogs(logLines, 1765163618);

      const container = wrapper.vm.$refs.logsContainer;
      expect(container.children.length).toBe(3);
    });

    test('renders OSC 8 hyperlinks in appended log messages as clickable links', () => {
      const wrapper = createWrapper();
      const logLines = [
        // OSC 8 hyperlink: ESC ] 8 ; ; <url> ESC \ <text> ESC ] 8 ; ; ESC \
        {index: 1, timestamp: 1765163618, message: 'See \x1b]8;;https://example.com/build\x1b\\the build\x1b]8;;\x1b\\'},
      ];

      wrapper.vm.appendLogs(logLines, 1765163618);

      const logMessage = wrapper.get('.log-msg');
      const link = logMessage.get('a');
      expect(link.attributes('href')).toEqual('https://example.com/build');
      expect(link.attributes('target')).toEqual('_blank');
      expect(link.attributes('rel')).toEqual('noopener noreferrer nofollow');
      expect(logMessage.text()).toEqual('See the build');
    });

    test('if ANSI renders empty line, skip line & line number', async () => {
      const wrapper = createWrapper({
        expanded: true,
      });
      const logLines = [
        {index: 1, message: '\u001b]9;4;3\u0007\r\u001bM\u001b[?2026l\u001b[?2026h\u001b[J', timestamp: 0},
        {index: 2, message: 'second line', timestamp: 0},
        {index: 3, message: '\u001b]9;4;3\u0007\r\u001bM\u001b[?2026l\u001b[J\u001b]9;4;0\u0007\u001b[?2026h\u001b[J\u001b]9;4;1;0\u0007\u001b[?2026l\u001b[J\u001b]9;4;0\u0007', timestamp: 0},
        {index: 4, message: 'fourth line', timestamp: 0},
      ];
      wrapper.vm.appendLogs(logLines, 1765163618);

      // Check if two lines where rendered
      expect(wrapper.findAll('.job-log-line').length).toEqual(2);

      // Check line one.
      expect(wrapper.get('.job-log-line:nth-of-type(1)').attributes('id')).toEqual('jobstep-12321-1');
      expect(wrapper.get('.job-log-line:nth-of-type(1) .line-num').text()).toEqual('1');
      expect(wrapper.get('.job-log-line:nth-of-type(1) .line-num').attributes('href')).toEqual('#jobstep-12321-1');
      expect(wrapper.get('.job-log-line:nth-of-type(1) .log-msg').text()).toEqual('second line');

      // Check line two.
      expect(wrapper.get('.job-log-line:nth-of-type(2)').attributes('id')).toEqual('jobstep-12321-2');
      expect(wrapper.get('.job-log-line:nth-of-type(2) .line-num').text()).toEqual('2');
      expect(wrapper.get('.job-log-line:nth-of-type(2) .line-num').attributes('href')).toEqual('#jobstep-12321-2');
      expect(wrapper.get('.job-log-line:nth-of-type(2) .log-msg').text()).toEqual('fourth line');
    });
  });

  describe('createLogLine method', () => {
    test('creates log line with correct structure', () => {
      const wrapper = createWrapper();
      const line = {
        index: 1,
        timestamp: 1765163618,
        message: 'Test message',
      };

      const logLine = wrapper.vm.createLogLine(line, 1765163618, {depth: 0, isHeader: false});

      expect(logLine.classList.contains('job-log-line')).toBe(true);
      expect(logLine.getAttribute('id')).toBe('jobstep-12321-1');
    });

    test('with timestamp', () => {
      const wrapper = createWrapper({timeVisibleTimestamp: true});
      const line = {
        index: 1,
        timestamp: 1765163618,
        message: 'Test message',
      };

      const logLine = wrapper.vm.createLogLine(line, 1765163618, {depth: 0, isHeader: false});

      expect(logLine.querySelector('.log-time-stamp').textContent).toBe('2025-12-08T03:13:38.000Z');
    });

    test('with duration', () => {
      const wrapper = createWrapper({timeVisibleSeconds: true});
      const line = {
        index: 1,
        timestamp: 1765163618,
        message: 'Test message',
      };

      const logLine = wrapper.vm.createLogLine(line, 1765163618 - 150, {depth: 0, isHeader: false});

      expect(logLine.querySelector('.log-time-seconds').textContent).toBe('150s');
    });

    test('creates line number link with correct href', () => {
      const wrapper = createWrapper();
      const line = {
        index: 5,
        timestamp: 1765163618,
        message: 'Test',
      };

      const logLine = wrapper.vm.createLogLine(line, 1765163618, {depth: 0, isHeader: false});
      const lineNumber = logLine.querySelector('.line-num');

      expect(lineNumber.textContent).toBe('5');
      expect(lineNumber.getAttribute('href')).toBe('#jobstep-12321-5');
    });
  });

  test('append logs with a group', () => {
    const lines = [
      {index: 1, message: '##[group]Test group', timestamp: 0},
      {index: 2, message: 'A test line', timestamp: 0},
      {index: 3, message: '##[endgroup]', timestamp: 0},
      {index: 4, message: 'A line outside the group', timestamp: 0},
    ];

    const wrapper = createWrapper();
    wrapper.vm.appendLogs(lines, 1765163618);

    // Check if 3 lines where rendered
    expect(wrapper.findAll('.job-log-line').length).toEqual(3);

    // Check if line 1 contains the group header
    expect(wrapper.get('.job-log-line:nth-of-type(1) > details.log-msg').text()).toEqual('Test group');

    // Check if right after the header line exists a log list
    expect(wrapper.find('.job-log-line:nth-of-type(1) + .job-log-list.hidden').exists()).toBe(true);

    // Check if inside the loglist exist exactly one log line
    expect(wrapper.findAll('.job-log-list > .job-log-line').length).toEqual(1);

    // Check if inside the loglist is an logline with our second logline
    expect(wrapper.get('.job-log-list > .job-log-line > .log-msg').text()).toEqual('A test line');

    // Check if after the log list exists another log line
    expect(wrapper.get('.job-log-list + .job-log-line > .log-msg').text()).toEqual('A line outside the group');
  });

  test('scrollIntoView focuses on a line from the log', () => {
    const wrapper = createWrapper();
    const logLines = [
      {index: 1, timestamp: 1765163618, message: 'Starting build'},
      {index: 2, timestamp: 1765163619, message: 'Running tests'},
      {index: 3, timestamp: 1765163620, message: 'Build complete'},
    ];
    wrapper.vm.appendLogs(logLines, 1765163618);

    const scrollIntoViewMock = vi.fn();
    const targetElement = wrapper.find('#jobstep-12321-1 .line-num').element;
    targetElement.scrollIntoView = scrollIntoViewMock;

    wrapper.vm.scrollIntoView('#jobstep-12321-1');

    expect(scrollIntoViewMock).toHaveBeenCalled();
  });
});
