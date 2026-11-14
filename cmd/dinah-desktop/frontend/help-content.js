/* This guide and its command reference describe the existing controls. */
(function () {
  'use strict';
  var commands = window.dinahCommands;
  window.dinahWorkbench.register('help-chats', {
    singleton: true, title: 'Working with Chats', glyph: 'info',
    init: function (root) {
      root.appendChild(document.getElementById('help-chats-content').content.cloneNode(true));
      return {};
    }
  });
  commands.register({ id: 'help-chats', menu: 'help', mnemonic: 'W', label: 'Working with Chats', help: { key: 'Use the Help menu' },
    run: function () { return window.dinahWorkbench.open('help-chats').catch(window.dinahHost.fail); } });
  commands.addHelp('View', [
    { label: 'Decisions', where: 'View menu or Decisions count in the status bar', key: 'Use the menu or status button' },
    { label: 'Broadcast to Active', where: 'View menu', key: 'Use the menu' },
    { label: 'Broadcast to All', where: 'View menu', key: 'Use the menu' }
  ], 'chat-decisions');
  commands.addHelp('Decisions', [
    { label: 'As Recommended', where: 'Beside an unanswered question', key: 'Tab to the button; press Enter or Space' },
    { label: 'Send Answers', where: 'Below the questions and reply destination', key: 'Tab to the button; press Enter or Space' },
    { label: 'Earlier Requests', where: 'Requests toolbar', key: 'Tab to the button; press Enter or Space' },
    { label: 'Preview Recipients', where: 'Broadcast composition', key: 'Tab to the button; press Enter or Space' },
    { label: 'Confirm Broadcast', where: 'After reviewing the recipient preview', key: 'Tab to the button; press Enter or Space' }
  ], 'decision-actions');
  commands.addHelp('Application', [
    { label: 'Settings', where: 'Application menu', key: 'Use the menu' },
    { label: 'Pause Chats and Restart', where: 'Application menu', key: 'Use the menu' },
    { label: 'Restart When Chats Finish', where: 'Application menu, when an update is installed', key: 'Use the menu' },
    { label: 'Restart Now', where: 'Restart notice in Notifications or the status bar', key: 'Use the notice button' },
    { label: 'Cancel the Restart', where: 'Application menu or restart notice, while a restart waits', key: 'Use the menu or notice button' }
  ], 'chat-restart');
  commands.addHelp('Settings', [
    { label: 'Maximum Chats', where: 'Chat, under Background Chats', key: 'Tab to the field; type a number' },
    { label: 'Check for Updates', where: 'Updates, when provided by this build', key: 'Tab to the button; press Enter or Space' },
    { label: 'Install Update', where: 'Updates, after a release is found', key: 'Tab to the button; press Enter or Space' },
    { label: 'Check Skill Packages', where: 'Updates', key: 'Tab to the button; press Enter or Space' },
    { label: 'Update Skill Package', where: 'Updates, beside an available package', key: 'Tab to the button; press Enter or Space' },
    { label: 'Check Agent Settings', where: 'Updates, before repairing settings', key: 'Tab to the button; press Enter or Space' },
    { label: 'Repair Agent Settings', where: 'Updates, after reviewing its preview', key: 'Tab to the button; press Enter or Space' }
  ], 'chat-settings');
}());
