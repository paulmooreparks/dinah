// Open a fixture's recent document through the supported Find and Open
// command, by its title. The fixture's RecentDocuments must list it.
module.exports = async function (page, title) {
  const listed = await page.evaluate(async title => {
    const recent = await window.go.main.App.RecentDocuments();
    return (recent || []).some(doc => doc.title === title);
  }, title);
  if (!listed) throw Error('Fixture document not in RecentDocuments: ' + title);
  await page.keyboard.press('Control+p');
  await page.locator('#go-to-filter').fill(title);
  await page.locator('#go-to-list tr').filter({ has: page.locator('span').getByText(title, { exact: true }) }).first().dblclick();
};
