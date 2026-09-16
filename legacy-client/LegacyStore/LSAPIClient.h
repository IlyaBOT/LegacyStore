#import <Cocoa/Cocoa.h>

@class LSAPIClient;

@protocol LSAPIClientDelegate
- (void)apiClient:(LSAPIClient *)client didFinishRequest:(NSString *)requestName object:(id)object;
- (void)apiClient:(LSAPIClient *)client didFailRequest:(NSString *)requestName error:(NSString *)errorMessage;
@end

@interface LSAPIClient : NSObject
{
    NSString *_baseURL;
    id _delegate;
    NSURLConnection *_connection;
    NSMutableData *_responseData;
    NSString *_requestName;
    int _statusCode;
}

- (id)initWithBaseURL:(NSString *)baseURL delegate:(id)delegate;
- (NSString *)baseURL;
- (void)setBaseURL:(NSString *)baseURL;
- (void)cancel;

- (void)loadBootstrap;
- (void)loadCategories;
- (void)loadAppsForCategory:(NSString *)category page:(int)page limit:(int)limit;
- (void)search:(NSString *)query page:(int)page limit:(int)limit;
- (void)loadAppWithSlug:(NSString *)slug;

@end
