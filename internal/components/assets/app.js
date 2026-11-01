htmx.config.includeIndicatorStyles = false;
htmx.config.responseHandling = [
  { code: "204", swap: false },
  { code: "[23]..", swap: true },
  { code: "[45]..", swap: true, error: true },
  { code: "...", swap: false },
];
