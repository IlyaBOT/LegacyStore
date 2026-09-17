#import "LSJSONParser.h"

@interface LSJSONParser (Private)
- (void)skipWhitespace;
- (id)parseValue;
- (NSDictionary *)parseObject;
- (NSArray *)parseArray;
- (NSString *)parseString;
- (NSNumber *)parseNumber;
- (BOOL)consumeLiteral:(NSString *)literal;
- (void)setError:(NSString *)message;
@end

@implementation LSJSONParser

- (id)initWithString:(NSString *)text
{
    self = [super init];
    if (self != nil) {
        _text = [text copy];
        _index = 0;
        _length = (unsigned int)[_text length];
        _errorMessage = nil;
    }
    return self;
}

- (void)dealloc
{
    [_text release];
    [_errorMessage release];
    [super dealloc];
}

- (NSString *)errorMessage
{
    return _errorMessage;
}

+ (id)objectWithData:(NSData *)data errorMessage:(NSString **)errorMessage
{
    NSString *text = [[[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding] autorelease];
    if (text == nil) {
        if (errorMessage != NULL) {
            *errorMessage = @"Response is not valid UTF-8.";
        }
        return nil;
    }

    LSJSONParser *parser = [[[LSJSONParser alloc] initWithString:text] autorelease];
    id object = [parser parse];
    if (object == nil && errorMessage != NULL) {
        *errorMessage = [parser errorMessage];
    }
    return object;
}

- (id)parse
{
    [self skipWhitespace];
    id result = [self parseValue];
    if (result == nil) {
        return nil;
    }
    [self skipWhitespace];
    if (_index != _length) {
        [self setError:@"Unexpected characters after JSON value."];
        return nil;
    }
    return result;
}

- (void)skipWhitespace
{
    while (_index < _length) {
        unichar c = [_text characterAtIndex:_index];
        if (c == ' ' || c == '\t' || c == '\r' || c == '\n') {
            _index++;
        } else {
            break;
        }
    }
}

- (id)parseValue
{
    [self skipWhitespace];
    if (_index >= _length) {
        [self setError:@"Unexpected end of JSON input."];
        return nil;
    }

    unichar c = [_text characterAtIndex:_index];
    if (c == '{') {
        return [self parseObject];
    }
    if (c == '[') {
        return [self parseArray];
    }
    if (c == '"') {
        return [self parseString];
    }
    if (c == '-' || (c >= '0' && c <= '9')) {
        return [self parseNumber];
    }
    if ([self consumeLiteral:@"true"]) {
        return [NSNumber numberWithBool:YES];
    }
    if ([self consumeLiteral:@"false"]) {
        return [NSNumber numberWithBool:NO];
    }
    if ([self consumeLiteral:@"null"]) {
        return [NSNull null];
    }

    [self setError:@"Unexpected JSON token."];
    return nil;
}

- (NSDictionary *)parseObject
{
    NSMutableDictionary *result = [NSMutableDictionary dictionary];
    _index++;
    [self skipWhitespace];

    if (_index < _length && [_text characterAtIndex:_index] == '}') {
        _index++;
        return result;
    }

    while (_index < _length) {
        [self skipWhitespace];
        if (_index >= _length || [_text characterAtIndex:_index] != '"') {
            [self setError:@"Expected JSON object key."];
            return nil;
        }

        NSString *key = [self parseString];
        if (key == nil) {
            return nil;
        }

        [self skipWhitespace];
        if (_index >= _length || [_text characterAtIndex:_index] != ':') {
            [self setError:@"Expected ':' after JSON object key."];
            return nil;
        }
        _index++;

        id value = [self parseValue];
        if (value == nil) {
            return nil;
        }
        [result setObject:value forKey:key];

        [self skipWhitespace];
        if (_index >= _length) {
            [self setError:@"Unexpected end of JSON object."];
            return nil;
        }
        unichar c = [_text characterAtIndex:_index++];
        if (c == '}') {
            return result;
        }
        if (c != ',') {
            [self setError:@"Expected ',' or '}' in JSON object."];
            return nil;
        }
    }

    [self setError:@"Unexpected end of JSON object."];
    return nil;
}

- (NSArray *)parseArray
{
    NSMutableArray *result = [NSMutableArray array];
    _index++;
    [self skipWhitespace];

    if (_index < _length && [_text characterAtIndex:_index] == ']') {
        _index++;
        return result;
    }

    while (_index < _length) {
        id value = [self parseValue];
        if (value == nil) {
            return nil;
        }
        [result addObject:value];

        [self skipWhitespace];
        if (_index >= _length) {
            [self setError:@"Unexpected end of JSON array."];
            return nil;
        }
        unichar c = [_text characterAtIndex:_index++];
        if (c == ']') {
            return result;
        }
        if (c != ',') {
            [self setError:@"Expected ',' or ']' in JSON array."];
            return nil;
        }
    }

    [self setError:@"Unexpected end of JSON array."];
    return nil;
}

- (NSString *)parseString
{
    if (_index >= _length || [_text characterAtIndex:_index] != '"') {
        [self setError:@"Expected JSON string."];
        return nil;
    }
    _index++;

    NSMutableString *result = [NSMutableString string];
    while (_index < _length) {
        unichar c = [_text characterAtIndex:_index++];
        if (c == '"') {
            return result;
        }
        if (c < 0x20) {
            [self setError:@"Control character in JSON string."];
            return nil;
        }
        if (c != '\\') {
            [result appendFormat:@"%C", c];
            continue;
        }

        if (_index >= _length) {
            [self setError:@"Incomplete JSON string escape."];
            return nil;
        }
        unichar escape = [_text characterAtIndex:_index++];
        switch (escape) {
            case '"': [result appendString:@"\""]; break;
            case '\\': [result appendString:@"\\"]; break;
            case '/': [result appendString:@"/"]; break;
            case 'b': [result appendFormat:@"%C", (unichar)0x08]; break;
            case 'f': [result appendFormat:@"%C", (unichar)0x0C]; break;
            case 'n': [result appendString:@"\n"]; break;
            case 'r': [result appendString:@"\r"]; break;
            case 't': [result appendString:@"\t"]; break;
            case 'u': {
                if (_index + 4 > _length) {
                    [self setError:@"Incomplete Unicode escape."];
                    return nil;
                }
                unsigned int value = 0;
                unsigned int i;
                for (i = 0; i < 4; i++) {
                    unichar h = [_text characterAtIndex:_index++];
                    value <<= 4;
                    if (h >= '0' && h <= '9') value += h - '0';
                    else if (h >= 'a' && h <= 'f') value += 10 + h - 'a';
                    else if (h >= 'A' && h <= 'F') value += 10 + h - 'A';
                    else {
                        [self setError:@"Invalid Unicode escape."];
                        return nil;
                    }
                }
                [result appendFormat:@"%C", (unichar)value];
                break;
            }
            default:
                [self setError:@"Invalid JSON string escape."];
                return nil;
        }
    }

    [self setError:@"Unterminated JSON string."];
    return nil;
}

- (NSNumber *)parseNumber
{
    unsigned int start = _index;
    BOOL floating = NO;

    if (_index < _length && [_text characterAtIndex:_index] == '-') {
        _index++;
    }
    while (_index < _length) {
        unichar c = [_text characterAtIndex:_index];
        if (c < '0' || c > '9') break;
        _index++;
    }
    if (_index < _length && [_text characterAtIndex:_index] == '.') {
        floating = YES;
        _index++;
        while (_index < _length) {
            unichar c = [_text characterAtIndex:_index];
            if (c < '0' || c > '9') break;
            _index++;
        }
    }
    if (_index < _length) {
        unichar c = [_text characterAtIndex:_index];
        if (c == 'e' || c == 'E') {
            floating = YES;
            _index++;
            if (_index < _length) {
                c = [_text characterAtIndex:_index];
                if (c == '+' || c == '-') _index++;
            }
            while (_index < _length) {
                c = [_text characterAtIndex:_index];
                if (c < '0' || c > '9') break;
                _index++;
            }
        }
    }

    if (_index == start) {
        [self setError:@"Invalid JSON number."];
        return nil;
    }

    NSString *number = [_text substringWithRange:NSMakeRange(start, _index - start)];
    if (floating) {
        return [NSNumber numberWithDouble:[number doubleValue]];
    }
    return [NSNumber numberWithLongLong:[number longLongValue]];
}

- (BOOL)consumeLiteral:(NSString *)literal
{
    unsigned int literalLength = (unsigned int)[literal length];
    if (_index + literalLength > _length) {
        return NO;
    }
    if ([[_text substringWithRange:NSMakeRange(_index, literalLength)] isEqualToString:literal]) {
        _index += literalLength;
        return YES;
    }
    return NO;
}

- (void)setError:(NSString *)message
{
    if (_errorMessage != nil) {
        return;
    }
    _errorMessage = [[NSString stringWithFormat:@"%@ Offset: %u", message, _index] retain];
}

@end
