#import "LSCompatibility.h"
#import "LSAPIClient.h"

@interface LSMainWindowController : NSWindowController <LSAPIClientDelegate>
{
    LSAPIClient *_apiClient;
    NSSplitView *_splitView;
    NSTableView *_sidebarTable;
    NSTableView *_appsTable;
    NSSearchField *_searchField;
    NSTextField *_statusField;
    NSMutableArray *_sidebarItems;
    NSMutableArray *_apps;
    NSString *_selectedCategory;
}

- (id)initWithServerURL:(NSString *)serverURL;
- (void)reloadCatalog:(id)sender;
- (void)searchChanged:(id)sender;
- (void)openSelectedApp:(id)sender;

@end
