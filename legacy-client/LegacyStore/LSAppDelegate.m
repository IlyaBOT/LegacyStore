#import "LSAppDelegate.h"
#import "LSMainWindowController.h"

@implementation LSAppDelegate

- (void)applicationDidFinishLaunching:(NSNotification *)notification
{
    NSString *serverURL = [[NSUserDefaults standardUserDefaults] stringForKey:@"LegacyStoreServerURL"];
    if (serverURL == nil || [serverURL length] == 0) {
        serverURL = @"http://localhost:8080";
    }

    _mainWindowController = [[LSMainWindowController alloc] initWithServerURL:serverURL];
    [_mainWindowController showWindow:self];
    [[_mainWindowController window] makeKeyAndOrderFront:self];
}

- (BOOL)applicationShouldTerminateAfterLastWindowClosed:(NSApplication *)sender
{
    return YES;
}

- (void)dealloc
{
    [_mainWindowController release];
    [super dealloc];
}

@end
