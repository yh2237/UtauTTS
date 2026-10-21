export default function (eleventyConfig) {
  eleventyConfig.setNunjucksEnvironmentOptions({
    trimBlocks: true,
    lstripBlocks: true,
  })

  eleventyConfig.ignores.add('vitepress/**')
  eleventyConfig.ignores.add('dist/**')

  for (const dir of ['css', 'js', 'img']) {
    eleventyConfig.addPassthroughCopy(dir)
  }

  return {
    dir: {
      input: '.',
      output: 'dist',
      includes: '_includes',
      data: '_data',
    },
    templateFormats: ['njk'],
    htmlTemplateEngine: 'njk',
    markdownTemplateEngine: 'njk',
  }
}
