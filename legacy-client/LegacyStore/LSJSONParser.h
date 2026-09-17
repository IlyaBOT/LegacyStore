#import <Cocoa/Cocoa.h>

@interface LSJSONParser : NSObject
{
    NSString *_text;
    unsigned int _index;
    unsigned int _length;
    NSString *_errorMessage;
}

- (id)initWithString:(NSString *)text;
- (id)parse;
- (NSString *)errorMessage;

+ (id)objectWithData:(NSData *)data errorMessage:(NSString **)errorMessage;
@end
