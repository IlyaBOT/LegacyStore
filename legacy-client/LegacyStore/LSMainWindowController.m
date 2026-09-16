#import "LSMainWindowController.h"
#import "LSSystemInfo.h"

static NSString *LSFeaturedToolbarItem = @"LegacyStoreFeatured";
static NSString *LSTopChartsToolbarItem = @"LegacyStoreTopCharts";
static NSString *LSCategoriesToolbarItem = @"LegacyStoreCategories";
static NSString *LSDownloadsToolbarItem = @"LegacyStoreDownloads";
static NSString *LSUpdatesToolbarItem = @"LegacyStoreUpdates";

@interface LSMainWindowController (Private)
- (void)buildWindow;
- (void)buildToolbar;
- (NSToolbarItem *)toolbarItemWithIdentifier:(NSString *)identifier label:(NSString *)label;
- (void)toolbarAction:(id)sender;
- (void)showCatalogStatus:(NSString *)text;
- (void)replaceApps:(NSArray *)items;
- (NSDictionary *)sidebarItemAtRow:(LSInteger)row;
@end

@implementation LSMainWindowController

- (id)initWithServerURL:(NSString *)serverURL
{
    self = [super initWithWindow:nil];
    if (self != nil) {
        _sidebarItems = [[NSMutableArray alloc] init];
        _apps = [[NSMutableArray alloc] init];
        _selectedCategory = nil;
        _apiClient = [[LSAPIClient alloc] initWithBaseURL:serverURL delegate:self];
        [self buildWindow];
    }
    return self;
}

- (void)dealloc
{
    [_apiClient cancel];
    [_apiClient release];
    [_sidebarItems release];
    [_apps release];
    [_selectedCategory release];
    [super dealloc];
}

- (void)buildWindow
{
    NSRect frame = NSMakeRect(0, 0, 920, 610);
    unsigned int style = NSTitledWindowMask | NSClosableWindowMask | NSMiniaturizableWindowMask | NSResizableWindowMask;
    NSWindow *window = [[[NSWindow alloc] initWithContentRect:frame
                                                   styleMask:style
                                                     backing:NSBackingStoreBuffered
                                                       defer:NO] autorelease];
    [window setTitle:@"LegacyStore"];
    [window setMinSize:NSMakeSize(720, 460)];
    [window center];
    [self setWindow:window];

    [self buildToolbar];

    NSView *content = [window contentView];
    NSRect bounds = [content bounds];

    _splitView = [[[NSSplitView alloc] initWithFrame:bounds] autorelease];
    [_splitView setAutoresizingMask:NSViewWidthSizable | NSViewHeightSizable];
    [_splitView setVertical:YES];
    [_splitView setDividerStyle:NSSplitViewDividerStyleThin];
    [content addSubview:_splitView];

    NSView *sidebarContainer = [[[NSView alloc] initWithFrame:NSMakeRect(0, 0, 188, bounds.size.height)] autorelease];
    NSScrollView *sidebarScroll = [[[NSScrollView alloc] initWithFrame:[sidebarContainer bounds]] autorelease];
    [sidebarScroll setAutoresizingMask:NSViewWidthSizable | NSViewHeightSizable];
    [sidebarScroll setHasVerticalScroller:YES];
    [sidebarScroll setBorderType:NSNoBorder];

    _sidebarTable = [[[NSTableView alloc] initWithFrame:[sidebarScroll bounds]] autorelease];
    NSTableColumn *sidebarColumn = [[[NSTableColumn alloc] initWithIdentifier:@"sidebar"] autorelease];
    [sidebarColumn setWidth:178];
    [[sidebarColumn headerCell] setStringValue:@""];
    [_sidebarTable addTableColumn:sidebarColumn];
    [_sidebarTable setHeaderView:nil];
    [_sidebarTable setDelegate:self];
    [_sidebarTable setDataSource:self];
    [_sidebarTable setAllowsMultipleSelection:NO];
    [_sidebarTable setRowHeight:22.0];
    [sidebarScroll setDocumentView:_sidebarTable];
    [sidebarContainer addSubview:sidebarScroll];

    NSView *mainContainer = [[[NSView alloc] initWithFrame:NSMakeRect(0, 0, bounds.size.width - 190, bounds.size.height)] autorelease];
    NSRect mainBounds = [mainContainer bounds];

    _searchField = [[[NSSearchField alloc] initWithFrame:NSMakeRect(mainBounds.size.width - 245, mainBounds.size.height - 34, 225, 22)] autorelease];
    [_searchField setAutoresizingMask:NSViewMinXMargin | NSViewMinYMargin];
    [_searchField setTarget:self];
    [_searchField setAction:@selector(searchChanged:)];
    [mainContainer addSubview:_searchField];

    NSTextField *title = [[[NSTextField alloc] initWithFrame:NSMakeRect(18, mainBounds.size.height - 36, 360, 24)] autorelease];
    [title setBordered:NO];
    [title setEditable:NO];
    [title setSelectable:NO];
    [title setDrawsBackground:NO];
    [title setFont:[NSFont boldSystemFontOfSize:18.0]];
    [title setStringValue:@"Featured"];
    [title setAutoresizingMask:NSViewMaxXMargin | NSViewMinYMargin];
    [mainContainer addSubview:title];

    NSScrollView *appsScroll = [[[NSScrollView alloc] initWithFrame:NSMakeRect(16, 42, mainBounds.size.width - 32, mainBounds.size.height - 88)] autorelease];
    [appsScroll setAutoresizingMask:NSViewWidthSizable | NSViewHeightSizable];
    [appsScroll setHasVerticalScroller:YES];
    [appsScroll setBorderType:NSBezelBorder];

    _appsTable = [[[NSTableView alloc] initWithFrame:[appsScroll bounds]] autorelease];
    [_appsTable setDelegate:self];
    [_appsTable setDataSource:self];
    [_appsTable setDoubleAction:@selector(openSelectedApp:)];
    [_appsTable setTarget:self];
    [_appsTable setRowHeight:36.0];

    NSTableColumn *nameColumn = [[[NSTableColumn alloc] initWithIdentifier:@"name"] autorelease];
    [nameColumn setWidth:230];
    [[nameColumn headerCell] setStringValue:@"Application"];
    [_appsTable addTableColumn:nameColumn];

    NSTableColumn *categoryColumn = [[[NSTableColumn alloc] initWithIdentifier:@"category"] autorelease];
    [categoryColumn setWidth:160];
    [[categoryColumn headerCell] setStringValue:@"Category"];
    [_appsTable addTableColumn:categoryColumn];

    NSTableColumn *versionColumn = [[[NSTableColumn alloc] initWithIdentifier:@"version"] autorelease];
    [versionColumn setWidth:95];
    [[versionColumn headerCell] setStringValue:@"Version"];
    [_appsTable addTableColumn:versionColumn];

    NSTableColumn *statusColumn = [[[NSTableColumn alloc] initWithIdentifier:@"status"] autorelease];
    [statusColumn setWidth:130];
    [[statusColumn headerCell] setStringValue:@"Compatibility"];
    [_appsTable addTableColumn:statusColumn];

    [appsScroll setDocumentView:_appsTable];
    [mainContainer addSubview:appsScroll];

    _statusField = [[[NSTextField alloc] initWithFrame:NSMakeRect(18, 13, mainBounds.size.width - 36, 18)] autorelease];
    [_statusField setAutoresizingMask:NSViewWidthSizable | NSViewMaxYMargin];
    [_statusField setBordered:NO];
    [_statusField setEditable:NO];
    [_statusField setSelectable:NO];
    [_statusField setDrawsBackground:NO];
    [_statusField setFont:[NSFont systemFontOfSize:11.0]];
    [_statusField setStringValue:[NSString stringWithFormat:@"Mac OS X %@, %@", [LSSystemInfo systemVersion], [LSSystemInfo preferredCatalogArchitecture]]];
    [mainContainer addSubview:_statusField];

    [_splitView addSubview:sidebarContainer];
    [_splitView addSubview:mainContainer];
    [_splitView setPosition:188.0 ofDividerAtIndex:0];

    [_sidebarItems addObject:[NSDictionary dictionaryWithObjectsAndKeys:@"All Software", @"name", @"", @"slug", nil]];
    [_sidebarTable reloadData];
    [_sidebarTable selectRowIndexes:[NSIndexSet indexSetWithIndex:0] byExtendingSelection:NO];

    [self showCatalogStatus:@"Connecting to LegacyStore..."];
    [_apiClient loadCategories];
}

- (void)buildToolbar
{
    NSToolbar *toolbar = [[[NSToolbar alloc] initWithIdentifier:@"LegacyStoreMainToolbar"] autorelease];
    [toolbar setDelegate:self];
    [toolbar setAllowsUserCustomization:NO];
    [toolbar setAutosavesConfiguration:NO];
    [toolbar setDisplayMode:NSToolbarDisplayModeIconAndLabel];
    [[self window] setToolbar:toolbar];
}

- (NSToolbarItem *)toolbarItemWithIdentifier:(NSString *)identifier label:(NSString *)label
{
    NSToolbarItem *item = [[[NSToolbarItem alloc] initWithItemIdentifier:identifier] autorelease];
    [item setLabel:label];
    [item setPaletteLabel:label];
    [item setToolTip:label];
    [item setTarget:self];
    [item setAction:@selector(toolbarAction:)];
    return item;
}

- (NSArray *)toolbarAllowedItemIdentifiers:(NSToolbar *)toolbar
{
    return [NSArray arrayWithObjects:LSFeaturedToolbarItem, LSTopChartsToolbarItem, LSCategoriesToolbarItem,
            LSDownloadsToolbarItem, LSUpdatesToolbarItem, NSToolbarFlexibleSpaceItemIdentifier, nil];
}

- (NSArray *)toolbarDefaultItemIdentifiers:(NSToolbar *)toolbar
{
    return [NSArray arrayWithObjects:LSFeaturedToolbarItem, LSTopChartsToolbarItem, LSCategoriesToolbarItem,
            NSToolbarFlexibleSpaceItemIdentifier, LSDownloadsToolbarItem, LSUpdatesToolbarItem, nil];
}

- (NSToolbarItem *)toolbar:(NSToolbar *)toolbar itemForItemIdentifier:(NSString *)identifier willBeInsertedIntoToolbar:(BOOL)flag
{
    if ([identifier isEqualToString:LSFeaturedToolbarItem]) return [self toolbarItemWithIdentifier:identifier label:@"Featured"];
    if ([identifier isEqualToString:LSTopChartsToolbarItem]) return [self toolbarItemWithIdentifier:identifier label:@"Top Charts"];
    if ([identifier isEqualToString:LSCategoriesToolbarItem]) return [self toolbarItemWithIdentifier:identifier label:@"Categories"];
    if ([identifier isEqualToString:LSDownloadsToolbarItem]) return [self toolbarItemWithIdentifier:identifier label:@"Downloads"];
    if ([identifier isEqualToString:LSUpdatesToolbarItem]) return [self toolbarItemWithIdentifier:identifier label:@"Updates"];
    return nil;
}

- (void)toolbarAction:(id)sender
{
    NSString *identifier = [sender itemIdentifier];
    if ([identifier isEqualToString:LSFeaturedToolbarItem] || [identifier isEqualToString:LSCategoriesToolbarItem]) {
        [self reloadCatalog:sender];
        return;
    }
    if ([identifier isEqualToString:LSTopChartsToolbarItem]) {
        [self showCatalogStatus:@"Top Charts will use the catalog ranking endpoint in a later iteration."];
        return;
    }
    if ([identifier isEqualToString:LSDownloadsToolbarItem]) {
        [self showCatalogStatus:@"Download manager is not enabled in this build yet."];
        return;
    }
    if ([identifier isEqualToString:LSUpdatesToolbarItem]) {
        [self showCatalogStatus:@"Installed application scanning is not enabled in this build yet."];
    }
}

- (void)reloadCatalog:(id)sender
{
    [self showCatalogStatus:@"Loading catalog..."];
    [_apiClient loadAppsForCategory:_selectedCategory page:1 limit:50];
}

- (void)searchChanged:(id)sender
{
    NSString *query = [_searchField stringValue];
    if (query == nil || [query length] == 0) {
        [self reloadCatalog:sender];
        return;
    }
    [self showCatalogStatus:@"Searching..."];
    [_apiClient search:query page:1 limit:50];
}

- (void)openSelectedApp:(id)sender
{
    int row = (int)[_appsTable selectedRow];
    if (row < 0 || row >= (int)[_apps count]) {
        return;
    }
    NSDictionary *app = [_apps objectAtIndex:(unsigned int)row];
    NSString *slug = [app objectForKey:@"slug"];
    if (slug != nil) {
        [self showCatalogStatus:@"Loading application details..."];
        [_apiClient loadAppWithSlug:slug];
    }
}

- (LSInteger)numberOfRowsInTableView:(NSTableView *)tableView
{
    if (tableView == _sidebarTable) {
        return (LSInteger)[_sidebarItems count];
    }
    return (LSInteger)[_apps count];
}

- (id)tableView:(NSTableView *)tableView objectValueForTableColumn:(NSTableColumn *)tableColumn row:(LSInteger)row
{
    if (tableView == _sidebarTable) {
        NSDictionary *item = [self sidebarItemAtRow:row];
        return [item objectForKey:@"name"];
    }

    if (row < 0 || (unsigned int)row >= [_apps count]) {
        return @"";
    }
    NSDictionary *app = [_apps objectAtIndex:(unsigned int)row];
    NSString *identifier = [tableColumn identifier];
    if ([identifier isEqualToString:@"name"]) return [app objectForKey:@"name"];
    if ([identifier isEqualToString:@"category"]) return [app objectForKey:@"category"];
    if ([identifier isEqualToString:@"version"]) {
        NSString *version = [app objectForKey:@"recommended_version"];
        return version != nil ? version : @"—";
    }
    if ([identifier isEqualToString:@"status"]) {
        NSDictionary *compatibility = [app objectForKey:@"compatibility"];
        NSString *label = [compatibility objectForKey:@"label"];
        return label != nil ? label : @"Unknown";
    }
    return @"";
}

- (void)tableViewSelectionDidChange:(NSNotification *)notification
{
    if ([notification object] != _sidebarTable) {
        return;
    }
    int row = (int)[_sidebarTable selectedRow];
    if (row < 0 || row >= (int)[_sidebarItems count]) {
        return;
    }
    NSDictionary *item = [self sidebarItemAtRow:row];
    NSString *slug = [item objectForKey:@"slug"];
    [_selectedCategory release];
    _selectedCategory = [slug length] > 0 ? [slug copy] : nil;
    [_searchField setStringValue:@""];
    [self reloadCatalog:nil];
}

- (NSDictionary *)sidebarItemAtRow:(LSInteger)row
{
    if (row < 0 || (unsigned int)row >= [_sidebarItems count]) {
        return nil;
    }
    return [_sidebarItems objectAtIndex:(unsigned int)row];
}

- (void)replaceApps:(NSArray *)items
{
    [_apps removeAllObjects];
    if (items != nil) {
        [_apps addObjectsFromArray:items];
    }
    [_appsTable reloadData];
}

- (void)showCatalogStatus:(NSString *)text
{
    [_statusField setStringValue:text != nil ? text : @""];
}

- (void)apiClient:(LSAPIClient *)client didFinishRequest:(NSString *)requestName object:(id)object
{
    if ([requestName isEqualToString:@"categories"]) {
        NSArray *categories = [object objectForKey:@"categories"];
        NSEnumerator *enumerator = [categories objectEnumerator];
        NSDictionary *category;
        while ((category = [enumerator nextObject]) != nil) {
            NSString *name = [category objectForKey:@"name"];
            NSString *slug = [category objectForKey:@"slug"];
            if (name != nil && slug != nil) {
                [_sidebarItems addObject:[NSDictionary dictionaryWithObjectsAndKeys:name, @"name", slug, @"slug", nil]];
            }
        }
        [_sidebarTable reloadData];
        [self reloadCatalog:nil];
        return;
    }

    if ([requestName isEqualToString:@"apps"]) {
        [self replaceApps:[object objectForKey:@"apps"]];
        [self showCatalogStatus:[NSString stringWithFormat:@"Loaded %u applications for Mac OS X %@ (%@).",
                                 (unsigned int)[_apps count], [LSSystemInfo systemVersion], [LSSystemInfo preferredCatalogArchitecture]]];
        return;
    }

    if ([requestName isEqualToString:@"search"]) {
        [self replaceApps:[object objectForKey:@"results"]];
        [self showCatalogStatus:[NSString stringWithFormat:@"Found %u applications.", (unsigned int)[_apps count]]];
        return;
    }

    if ([requestName isEqualToString:@"app-detail"]) {
        NSString *name = [object objectForKey:@"name"];
        NSString *summary = [object objectForKey:@"summary"];
        NSDictionary *artifact = [object objectForKey:@"recommended_artifact"];
        NSDictionary *compatibility = [object objectForKey:@"compatibility"];
        NSString *version = [artifact objectForKey:@"version"];
        NSString *compatibilityLabel = [compatibility objectForKey:@"label"];
        NSString *message = [NSString stringWithFormat:@"%@\n\nVersion: %@\nCompatibility: %@",
                             summary != nil ? summary : @"",
                             version != nil ? version : @"No compatible version",
                             compatibilityLabel != nil ? compatibilityLabel : @"Unknown"];
        NSRunAlertPanel(name != nil ? name : @"LegacyStore", message, @"OK", nil, nil);
        [self showCatalogStatus:@"Application details loaded."];
        return;
    }

    [self showCatalogStatus:@"Request completed."];
}

- (void)apiClient:(LSAPIClient *)client didFailRequest:(NSString *)requestName error:(NSString *)errorMessage
{
    [self showCatalogStatus:[NSString stringWithFormat:@"%@ failed: %@", requestName, errorMessage]];
}

@end
