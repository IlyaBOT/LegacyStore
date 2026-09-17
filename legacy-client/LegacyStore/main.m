#import <Cocoa/Cocoa.h>
#import "LSAppDelegate.h"

int main(int argc, const char *argv[])
{
    NSAutoreleasePool *pool = [[NSAutoreleasePool alloc] init];
    NSApplication *application = [NSApplication sharedApplication];
    LSAppDelegate *delegate = [[LSAppDelegate alloc] init];

    [application setDelegate:delegate];
    [application run];

    [delegate release];
    [pool release];
    return 0;
}
