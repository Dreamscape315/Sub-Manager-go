/** Tailwind config used once (locally) to (re)generate static/css/tailwind.css
 * via the standalone CLI. Not part of the Go build; see README for the
 * regeneration command. */
module.exports = {
  content: ['./internal/web/templates/**/*.html'],
  theme: {
    extend: {
      fontFamily: {
        sans: [
          '-apple-system', 'BlinkMacSystemFont', '"Segoe UI"', 'Roboto',
          '"PingFang SC"', '"Hiragino Sans GB"', '"Microsoft YaHei"',
          '"Helvetica Neue"', 'sans-serif'
        ],
        mono: [
          'ui-monospace', 'SFMono-Regular', 'Menlo', 'Consolas', 'monospace'
        ]
      }
    }
  },
  plugins: []
}
