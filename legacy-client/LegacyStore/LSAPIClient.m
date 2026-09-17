#import "LSAPIClient.h"
#import "LSJSONParser.h"
#import "LSSystemInfo.h"

@interface LSAPIClient (Private)
- (void)startGET:(NSString *)path requestName:(NSString *)requestName;
- (NSString *)targetQuery;
- (NSString *)escape:(NSString *)value;
- (void)finishWithError:(NSString *)message;
@end

@implementation LSAPIClient

- (id)initWithBaseURL:(NSString *)baseURL delegate:(id)delegate
{
    self = [super init];
    if (self != nil) {
        _baseURL = [baseURL copy];
        _delegate = delegate;
        _connection = nil;
        _responseData = nil;
        _requestName = nil;
        _statusCode = 0;
    }
    return self;
}

- (void)dealloc
{
    [self cancel];
    [_baseURL release];
    [super dealloc];
}

- (NSString *)baseURL
{
    return _baseURL;
}

- (void)setBaseURL:(NSString *)baseURL
{
    if (_baseURL == baseURL || [_baseURL isEqualToString:baseURL]) {
        return;
    }
    [_baseURL release];
    _baseURL = [baseURL copy];
}

- (void)cancel
{
    if (_connection != nil) {
        [_connection cancel];
        [_connection release];
        _connection = nil;
    }
    [_responseData release];
    _responseData = nil;
    [_requestName release];
    _requestName = nil;
    _statusCode = 0;
}

- (void)loadBootstrap
{
    [self startGET:@"/api/v1/bootstrap" requestName:@"bootstrap"];
}

- (void)loadCategories
{
    [self startGET:@"/api/v1/categories" requestName:@"categories"];
}

- (void)loadAppsForCategory:(NSString *)category page:(int)page limit:(int)limit
{
    NSMutableString *path = [NSMutableString stringWithFormat:@"/api/v1/apps?%@&page=%d&limit=%d", [self targetQuery], page, limit];
    if (category != nil && [category length] > 0) {
        [path appendFormat:@"&category=%@", [self escape:category]];
    }
    [self startGET:path requestName:@"apps"];
}

- (void)search:(NSString *)query page:(int)page limit:(int)limit
{
    NSString *escaped = query != nil ? [self escape:query] : @"";
    NSString *path = [NSString stringWithFormat:@"/api/v1/search?q=%@&%@&page=%d&limit=%d", escaped, [self targetQuery], page, limit];
    [self startGET:path requestName:@"search"];
}

- (void)loadAppWithSlug:(NSString *)slug
{
    if (slug == nil || [slug length] == 0) {
        return;
    }
    NSString *path = [NSString stringWithFormat:@"/api/v1/apps/%@?%@", [self escape:slug], [self targetQuery]];
    [self startGET:path requestName:@"app-detail"];
}

- (NSString *)targetQuery
{
    return [NSString stringWithFormat:@"os=%@&arch=%@",
            [self escape:[LSSystemInfo systemVersion]],
            [self escape:[LSSystemInfo preferredCatalogArchitecture]]];
}

- (NSString *)escape:(NSString *)value
{
    NSString *escaped = [value stringByAddingPercentEscapesUsingEncoding:NSUTF8StringEncoding];
    return escaped != nil ? escaped : @"";
}

- (void)startGET:(NSString *)path requestName:(NSString *)requestName
{
    [self cancel];

    NSString *base = _baseURL;
    if ([base hasSuffix:@"/"]) {
        base = [base substringToIndex:[base length] - 1];
    }
    NSURL *url = [NSURL URLWithString:[base stringByAppendingString:path]];
    if (url == nil) {
        [self finishWithError:@"Invalid LegacyStore server URL."];
        return;
    }

    NSMutableURLRequest *request = [NSMutableURLRequest requestWithURL:url
                                                           cachePolicy:NSURLRequestReloadIgnoringCacheData
                                                       timeoutInterval:20.0];
    [request setHTTPMethod:@"GET"];
    [request setValue:@"application/json" forHTTPHeaderField:@"Accept"];
    [request setValue:@"LegacyStore/0.1" forHTTPHeaderField:@"User-Agent"];

    _requestName = [requestName copy];
    _responseData = [[NSMutableData alloc] init];
    _statusCode = 0;
    _connection = [[NSURLConnection alloc] initWithRequest:request delegate:self];
    if (_connection == nil) {
        [self finishWithError:@"Unable to start network request."];
    }
}

- (void)connection:(NSURLConnection *)connection didReceiveResponse:(NSURLResponse *)response
{
    [_responseData setLength:0];
    _statusCode = 0;
    if ([response isKindOfClass:[NSHTTPURLResponse class]]) {
        _statusCode = (int)[(NSHTTPURLResponse *)response statusCode];
    }
}

- (void)connection:(NSURLConnection *)connection didReceiveData:(NSData *)data
{
    [_responseData appendData:data];
}

- (void)connection:(NSURLConnection *)connection didFailWithError:(NSError *)error
{
    NSString *message = [error localizedDescription];
    if (message == nil) {
        message = @"Network request failed.";
    }
    [self finishWithError:message];
}

- (void)connectionDidFinishLoading:(NSURLConnection *)connection
{
    NSString *name = [[_requestName retain] autorelease];
    NSData *data = [[_responseData retain] autorelease];
    int status = _statusCode;

    [_connection release];
    _connection = nil;
    [_responseData release];
    _responseData = nil;
    [_requestName release];
    _requestName = nil;
    _statusCode = 0;

    if (status < 200 || status >= 300) {
        if (_delegate != nil && [_delegate respondsToSelector:@selector(apiClient:didFailRequest:error:)]) {
            [_delegate apiClient:self didFailRequest:name error:[NSString stringWithFormat:@"Server returned HTTP %d.", status]];
        }
        return;
    }

    NSString *parseError = nil;
    id object = [LSJSONParser objectWithData:data errorMessage:&parseError];
    if (object == nil) {
        if (_delegate != nil && [_delegate respondsToSelector:@selector(apiClient:didFailRequest:error:)]) {
            [_delegate apiClient:self didFailRequest:name error:parseError != nil ? parseError : @"Invalid JSON response."];
        }
        return;
    }

    if (_delegate != nil && [_delegate respondsToSelector:@selector(apiClient:didFinishRequest:object:)]) {
        [_delegate apiClient:self didFinishRequest:name object:object];
    }
}

- (void)finishWithError:(NSString *)message
{
    NSString *name = _requestName != nil ? [[_requestName copy] autorelease] : @"request";
    [self cancel];
    if (_delegate != nil && [_delegate respondsToSelector:@selector(apiClient:didFailRequest:error:)]) {
        [_delegate apiClient:self didFailRequest:name error:message];
    }
}

@end
