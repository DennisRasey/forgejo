import {DoUpdateRelativeTime, HALF_MINUTE, ONE_MINUTE, ONE_HOUR, ONE_DAY} from './relative-time.js';

test('CalculateRelativeTimes', () => {
  window.config.pageData.PLURAL_RULE_LANG = 0;
  window.config.pageData.DATETIMESTRINGS = {
    'FUTURE': 'in future',
    'NOW': 'now',
    'relativetime.1day': 'yesterday',
    'relativetime.1week': 'last week',
    'relativetime.1month': 'last month',
    'relativetime.1year': 'last year',
  };
  window.config.pageData.PLURALSTRINGS_LANG = {
    'relativetime.mins': ['%d minute ago', '%d minutes ago'],
    'relativetime.hours': ['%d hour ago', '%d hours ago'],
    'relativetime.days': ['%d day ago', '%d days ago'],
    'relativetime.weeks': ['%d week ago', '%d weeks ago'],
    'relativetime.months': ['%d month ago', '%d months ago'],
    'relativetime.years': ['%d year ago', '%d years ago'],
  };
  const mock = document.createElement('relative-time');
  document.body.append(mock);

  const now = Date.parse('2024-10-27T04:05:30+01:00');  // One hour after DST switchover, CET.

  expect(DoUpdateRelativeTime(mock, now)).toEqual(null);
  expect(mock.textContent).toEqual('');

  mock.setAttribute('datetime', '2024-10-27T04:06:40+01:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(70000);
  expect(mock.textContent).toEqual('in future');

  mock.setAttribute('datetime', '2024-10-27T04:05:10+01:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(HALF_MINUTE);
  expect(mock.textContent).toEqual('now');

  mock.setAttribute('datetime', '2024-10-27T04:04:30+01:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_MINUTE);
  expect(mock.textContent).toEqual('1 minute ago');

  mock.setAttribute('datetime', '2024-10-27T04:04:00+01:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_MINUTE);
  expect(mock.textContent).toEqual('1 minute ago');

  mock.setAttribute('datetime', '2024-10-27T04:03:20+01:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_MINUTE);
  expect(mock.textContent).toEqual('2 minutes ago');

  mock.setAttribute('datetime', '2024-10-27T04:00:00+01:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_MINUTE);
  expect(mock.textContent).toEqual('5 minutes ago');

  mock.setAttribute('datetime', '2024-10-27T03:59:30+01:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_MINUTE);
  expect(mock.textContent).toEqual('6 minutes ago');

  mock.setAttribute('datetime', '2024-10-27T03:01:00+01:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_HOUR);
  expect(mock.textContent).toEqual('1 hour ago');

  mock.setAttribute('datetime', '2024-10-27T01:00:00+02:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_HOUR);
  expect(mock.textContent).toEqual('4 hours ago');  // This tests DST switchover

  mock.setAttribute('datetime', '2024-10-27T00:01:00+02:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_HOUR);
  expect(mock.textContent).toEqual('5 hours ago');

  mock.setAttribute('datetime', '2024-10-26T22:00:00+02:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_HOUR);
  expect(mock.textContent).toEqual('7 hours ago');

  mock.setAttribute('datetime', '2024-10-26T05:08:00+02:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_HOUR);
  expect(mock.textContent).toEqual('23 hours ago');

  mock.setAttribute('datetime', '2024-10-26T04:00:00+02:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_DAY);
  expect(mock.textContent).toEqual('yesterday');

  mock.setAttribute('datetime', '2024-10-25T01:00:00+02:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_DAY);
  expect(mock.textContent).toEqual('2 days ago');

  mock.setAttribute('datetime', '2024-10-21T01:00:00+02:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_DAY);
  expect(mock.textContent).toEqual('6 days ago');

  mock.setAttribute('datetime', '2024-10-20T01:00:00+02:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_DAY);
  expect(mock.textContent).toEqual('last week');

  mock.setAttribute('datetime', '2024-10-14T01:00:00+02:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_DAY);
  expect(mock.textContent).toEqual('last week');

  mock.setAttribute('datetime', '2024-10-13T01:00:00+02:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_DAY);
  expect(mock.textContent).toEqual('2 weeks ago');

  mock.setAttribute('datetime', '2024-10-06T01:00:00+02:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_DAY);
  expect(mock.textContent).toEqual('3 weeks ago');

  mock.setAttribute('datetime', '2024-09-25T01:00:00+02:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_DAY);
  expect(mock.textContent).toEqual('last month');

  mock.setAttribute('datetime', '2024-08-30T01:00:00+02:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_DAY);
  expect(mock.textContent).toEqual('last month');

  mock.setAttribute('datetime', '2024-07-30T01:00:00+02:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_DAY);
  expect(mock.textContent).toEqual('2 months ago');

  mock.setAttribute('datetime', '2024-05-30T01:00:00+02:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_DAY);
  expect(mock.textContent).toEqual('4 months ago');

  mock.setAttribute('datetime', '2024-03-01T01:00:00+01:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_DAY);
  expect(mock.textContent).toEqual('7 months ago');

  mock.setAttribute('datetime', '2024-02-29T01:00:00+01:00');  // Leap day handling
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_DAY);
  expect(mock.textContent).toEqual('7 months ago');

  mock.setAttribute('datetime', '2024-02-27T01:00:00-03:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_DAY);
  expect(mock.textContent).toEqual('7 months ago');

  mock.setAttribute('datetime', '2024-02-27T01:00:00+01:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_DAY);
  expect(mock.textContent).toEqual('8 months ago');

  mock.setAttribute('datetime', '2023-11-15T01:00:00+03:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_DAY);
  expect(mock.textContent).toEqual('11 months ago');

  mock.setAttribute('datetime', '2023-10-20T01:00:00+08:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_DAY);
  expect(mock.textContent).toEqual('last year');

  mock.setAttribute('datetime', '2022-10-30T01:00:00-05:30');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_DAY);
  expect(mock.textContent).toEqual('last year');

  mock.setAttribute('datetime', '2022-10-20T01:00:00+10:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_DAY);
  expect(mock.textContent).toEqual('2 years ago');

  mock.setAttribute('datetime', '2021-10-20T01:00:00+02:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_DAY);
  expect(mock.textContent).toEqual('3 years ago');

  mock.setAttribute('datetime', '2014-10-20T01:00:00+02:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_DAY);
  expect(mock.textContent).toEqual('10 years ago');

  // Timezone tests
  mock.setAttribute('datetime', '2024-10-27T01:05:30-05:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(3 * ONE_HOUR);
  expect(mock.textContent).toEqual('in future');

  mock.setAttribute('datetime', '2024-10-27T05:05:25+02:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(HALF_MINUTE);
  expect(mock.textContent).toEqual('now');

  mock.setAttribute('datetime', '2024-10-27T05:04:00+02:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_MINUTE);
  expect(mock.textContent).toEqual('1 minute ago');

  mock.setAttribute('datetime', '2024-10-27T05:02:00+02:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_MINUTE);
  expect(mock.textContent).toEqual('3 minutes ago');

  mock.setAttribute('datetime', '2024-10-27T04:06:00+02:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_MINUTE);
  expect(mock.textContent).toEqual('59 minutes ago');

  mock.setAttribute('datetime', '2024-10-27T04:05:00+02:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_HOUR);
  expect(mock.textContent).toEqual('1 hour ago');

  mock.setAttribute('datetime', '2024-10-27T01:00:00+02:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_HOUR);
  expect(mock.textContent).toEqual('4 hours ago');

  mock.setAttribute('datetime', '2024-10-27T01:00:00+04:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_HOUR);
  expect(mock.textContent).toEqual('6 hours ago');

  mock.setAttribute('datetime', '2024-10-27T01:00:00+10:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_HOUR);
  expect(mock.textContent).toEqual('12 hours ago');

  mock.setAttribute('datetime', '2024-10-27T01:00:00Z');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_HOUR);
  expect(mock.textContent).toEqual('2 hours ago');

  mock.setAttribute('datetime', '2024-10-26T15:00:00-01:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_HOUR);
  expect(mock.textContent).toEqual('11 hours ago');

  mock.setAttribute('datetime', '2024-10-25T19:00:00-11:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_HOUR);
  expect(mock.textContent).toEqual('21 hours ago');
});

test('CalculateDurationFormat', () => {
  window.config.pageData.PLURAL_RULE_LANG = 0;
  // The suffix-free duration strings, mirroring options/locale_next translations.
  const durationStrings = {
    second: ['%d second', '%d seconds'],
    minute: ['%d minute', '%d minutes'],
    hour: ['%d hour', '%d hours'],
    day: ['%d day', '%d days'],
    week: ['%d week', '%d weeks'],
    month: ['%d month', '%d months'],
    year: ['%d year', '%d years'],
  };
  window.config.pageData.PLURALSTRINGS_LANG = {
    'relativetime.duration.secs': durationStrings.second,
    'relativetime.duration.mins': durationStrings.minute,
    'relativetime.duration.hours': durationStrings.hour,
    'relativetime.duration.days': durationStrings.day,
    'relativetime.duration.weeks': durationStrings.week,
    'relativetime.duration.months': durationStrings.month,
    'relativetime.duration.years': durationStrings.year,
  };

  // Build expected strings from the seeded translations, joined with the same
  // locale list formatter the component uses (plural rule 0: one iff n === 1).
  const listFmt = new Intl.ListFormat(navigator.language, {style: 'long', type: 'unit'});
  const unitStr = (n, unit) => durationStrings[unit][n === 1 ? 0 : 1].replace('%d', n);
  const fmt = (...pairs) => listFmt.format(pairs.map(([n, unit]) => unitStr(n, unit)));

  const mock = document.createElement('relative-time');
  document.body.append(mock);
  mock.setAttribute('format', 'duration');

  const now = Date.parse('2024-10-27T04:05:30+01:00');

  // Server uptime of just over two months: must render as "2 months, 2 days",
  // not "2 months ago".
  mock.setAttribute('datetime', '2024-08-25T01:00:00+02:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_DAY);
  expect(mock.textContent).toEqual(fmt([2, 'month'], [2, 'day']));

  // One year, some days remainder.
  mock.setAttribute('datetime', '2023-08-25T01:00:00+02:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_DAY);
  expect(mock.textContent).toEqual(fmt([1, 'year'], [63, 'day']));

  // Exactly one month, no remainder days — single unit.
  mock.setAttribute('datetime', '2024-09-27T04:05:30+01:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_DAY);
  expect(mock.textContent).toEqual(fmt([1, 'month']));

  // Days with hours remainder.
  mock.setAttribute('datetime', '2024-10-22T03:05:30+01:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_HOUR);
  expect(mock.textContent).toEqual(fmt([5, 'day'], [1, 'hour']));

  // Hours with minutes remainder — refresh once a minute.
  mock.setAttribute('datetime', '2024-10-27T01:00:30+01:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_MINUTE);
  expect(mock.textContent).toEqual(fmt([3, 'hour'], [5, 'minute']));

  // Minutes with seconds remainder.
  mock.setAttribute('datetime', '2024-10-27T04:00:00+01:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(HALF_MINUTE);
  expect(mock.textContent).toEqual(fmt([5, 'minute'], [30, 'second']));

  // Sub-minute durations should still be expressed as a duration, not "now".
  mock.setAttribute('datetime', '2024-10-27T04:05:20+01:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(HALF_MINUTE);
  expect(mock.textContent).toEqual(fmt([10, 'second']));

  // Future datetime should still produce a positive (absolute) duration.
  mock.setAttribute('datetime', '2024-10-27T04:08:30+01:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(HALF_MINUTE);
  expect(mock.textContent).toEqual(fmt([3, 'minute']));

  // Untranslated unit falls back to the browser's Intl formatting (no "ago").
  delete window.config.pageData.PLURALSTRINGS_LANG['relativetime.duration.months'];
  const intlFallback = new Intl.NumberFormat(navigator.language, {
    style: 'unit', unit: 'month', unitDisplay: 'long',
  }).format(2);
  mock.setAttribute('datetime', '2024-08-27T04:05:30+01:00');
  expect(DoUpdateRelativeTime(mock, now)).toEqual(ONE_DAY);
  expect(mock.textContent).toEqual(intlFallback);
});
