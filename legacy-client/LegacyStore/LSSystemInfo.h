#import <Cocoa/Cocoa.h>

@interface LSSystemInfo : NSObject
+ (NSString *)systemVersion;
+ (NSString *)processArchitecture;
+ (NSString *)preferredCatalogArchitecture;
+ (BOOL)cpuSupports64Bit;
+ (BOOL)supports32BitApplications;
+ (NSString *)machineModel;
+ (NSDictionary *)catalogTargetDictionary;
+ (NSComparisonResult)compareVersion:(NSString *)left toVersion:(NSString *)right;
@end
