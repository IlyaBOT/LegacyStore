#import "LSSystemInfo.h"
#import <sys/types.h>
#import <sys/sysctl.h>

@implementation LSSystemInfo

+ (NSString *)systemVersion
{
    NSDictionary *info = [NSDictionary dictionaryWithContentsOfFile:@"/System/Library/CoreServices/SystemVersion.plist"];
    NSString *version = [info objectForKey:@"ProductVersion"];
    if (version == nil || [version length] == 0) {
        return @"10.4";
    }
    return version;
}

+ (NSString *)processArchitecture
{
#if defined(__x86_64__)
    return @"x86_64";
#else
    return @"i386";
#endif
}

+ (BOOL)cpuSupports64Bit
{
    int value = 0;
    size_t size = sizeof(value);
    if (sysctlbyname("hw.cpu64bit_capable", &value, &size, NULL, 0) == 0) {
        return value != 0;
    }

    value = 0;
    size = sizeof(value);
    if (sysctlbyname("hw.optional.x86_64", &value, &size, NULL, 0) == 0) {
        return value != 0;
    }
    return NO;
}

+ (NSString *)preferredCatalogArchitecture
{
    if ([self cpuSupports64Bit] && [self compareVersion:[self systemVersion] toVersion:@"10.5"] != NSOrderedAscending) {
        return @"x86_64";
    }
    return @"i386";
}

+ (BOOL)supports32BitApplications
{
    return [self compareVersion:[self systemVersion] toVersion:@"10.15"] == NSOrderedAscending;
}

+ (NSString *)machineModel
{
    size_t size = 0;
    if (sysctlbyname("hw.model", NULL, &size, NULL, 0) != 0 || size == 0) {
        return @"UnknownMac";
    }

    char *buffer = (char *)malloc(size);
    if (buffer == NULL) {
        return @"UnknownMac";
    }
    if (sysctlbyname("hw.model", buffer, &size, NULL, 0) != 0) {
        free(buffer);
        return @"UnknownMac";
    }

    NSString *model = [NSString stringWithCString:buffer encoding:NSUTF8StringEncoding];
    free(buffer);
    return model != nil ? model : @"UnknownMac";
}

+ (NSDictionary *)catalogTargetDictionary
{
    return [NSDictionary dictionaryWithObjectsAndKeys:
            [self systemVersion], @"os_version",
            [self preferredCatalogArchitecture], @"arch",
            [NSNumber numberWithBool:[self supports32BitApplications]], @"supports_32bit",
            [NSNumber numberWithBool:[self cpuSupports64Bit]], @"supports_64bit",
            [self machineModel], @"machine_model",
            nil];
}

+ (NSComparisonResult)compareVersion:(NSString *)left toVersion:(NSString *)right
{
    NSArray *leftParts = [left componentsSeparatedByString:@"."];
    NSArray *rightParts = [right componentsSeparatedByString:@"."];
    unsigned int count = (unsigned int)MAX([leftParts count], [rightParts count]);
    unsigned int index;

    for (index = 0; index < count; index++) {
        int a = index < [leftParts count] ? [[leftParts objectAtIndex:index] intValue] : 0;
        int b = index < [rightParts count] ? [[rightParts objectAtIndex:index] intValue] : 0;
        if (a < b) {
            return NSOrderedAscending;
        }
        if (a > b) {
            return NSOrderedDescending;
        }
    }
    return NSOrderedSame;
}

@end
